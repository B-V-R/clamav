install:
	go install 
bin:
	@mkdir -p releases
	go get -v . && go build -o releases/clamav

test:
	@go get -v . 
	@go vet && go test -v .

container: build
	@docker build -f Dockerfile.release.yaml -t vighnesh.org/clamav:latest .

container-onbuild:
	@docker build -t vighnesh.org/clamav:onbuild .

push: container
	@docker push vighnesh.org/clamav

build: test
	go get -v . 														&& \
	mkdir -p releases													&& \
	GOARCH=386 GOOS=linux   go build -o releases/mav-linux		&& \
	GOARCH=386 GOOS=windows go build -o releases/mav-win.exe	&& \
	GOARCH=386 GOOS=darwin  go build -o releases/mav-darwin

clean:
	rm -rf ./releases | true

forma:
	@go fmt .
