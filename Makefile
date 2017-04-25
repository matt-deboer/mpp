VERSION  := $$(git describe --tags --always)
TARGET   := $$(basename `git rev-parse --show-toplevel`)
TEST     ?= $$(go list ./... | grep -v /vendor/)

default: test build

test:
	go test -v -cover -run=${RUN} ${TEST}

build: clean
	@go build -v -o bin/${TARGET} ./pkg/server

release: clean
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build \
		-a -tags netgo \
		-a -installsuffix cgo \
    -ldflags "-s -X main.Version=${VERSION} -X main.Name=${TARGET}" \
		-o bin/$(TARGET) ./pkg/server

clean:
	@rm -rf bin/