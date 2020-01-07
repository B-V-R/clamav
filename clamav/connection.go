package clamav

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const CHUNK_SIZE = 10*1024
const TCP_TIMEOUT = time.Second * 2

var resultRegex = regexp.MustCompile(
	`^(?P<path>[^:]+): ((?P<desc>[^:]+)(\((?P<virhash>([^:]+)):(?P<virsize>\d+)\))? )?(?P<status>FOUND|ERROR|OK)$`,
)

type Connection struct {
	net.Conn
}

func (conn *Connection) sendCommand(command string) error {
	commandBytes := []byte(fmt.Sprintf("n%s\n", command))

	_, err := conn.Write(commandBytes)
	return err
}

func (conn *Connection) sendEOF() error {
	_, err := conn.Write([]byte{0, 0, 0, 0})
	return err
}

func (conn *Connection) sendChunk(data []byte) error {
	var buf [4]byte
	lenData := len(data)
	buf[0] = byte(lenData >> 24)
	buf[1] = byte(lenData >> 16)
	buf[2] = byte(lenData >> 8)
	buf[3] = byte(lenData >> 0)

	a := buf

	b := make([]byte, len(a))
	for i := range a {
		b[i] = a[i]
	}

	conn.Write(b)

	_, err := conn.Write(data)
	return err
}

func (c *Connection) readResponse() (chan *ScanResult, *sync.WaitGroup, error) {
	var wg sync.WaitGroup

	wg.Add(1)
	reader := bufio.NewReader(c)
	ch := make(chan *ScanResult)

	go func() {
		defer func() {
			close(ch)
			wg.Done()
		}()

		for {
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				return
			}

			if err != nil {
				return
			}

			line = strings.TrimRight(line, " \t\r\n")
			ch <- parseResult(line)
		}
	}()

	return ch, &wg, nil
}

func parseResult(line string) *ScanResult {
	res := &ScanResult{}
	res.Raw = line

	matches := resultRegex.FindStringSubmatch(line)
	if len(matches) == 0 {
		res.Description = "Regex had no matches"
		res.Status = RES_PARSE_ERROR
		return res
	}

	for i, name := range resultRegex.SubexpNames() {
		switch name {
		case "path":
			res.Path = matches[i]
		case "desc":
			res.Description = matches[i]
		case "virhash":
			res.Hash = matches[i]
		case "virsize":
			i, err := strconv.Atoi(matches[i])
			if err == nil {
				res.Size = i
			}
		case "status":
			switch matches[i] {
			case RES_OK:
			case RES_FOUND:
			case RES_ERROR:
				break
			default:
				res.Description = "Invalid status field: " + matches[i]
				res.Status = RES_PARSE_ERROR
				return res
			}
			res.Status = matches[i]
		}
	}

	return res
}

func newCLAMDTcpConn(address string) (*Connection, error) {
	conn, err := net.DialTimeout("tcp", address, TCP_TIMEOUT)

	if err != nil {
		if nerr, isOk := err.(net.Error); isOk && nerr.Timeout() {
			return nil, nerr
		}

		return nil, err
	}

	return &Connection{Conn: conn}, err
}

func newCLAMDUnixConn(address string) (*Connection, error) {
	conn, err := net.Dial("unix", address)
	if err != nil {
		return nil, err
	}

	return &Connection{Conn: conn}, err
}