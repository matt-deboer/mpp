VERSION       ?= $(shell git describe --tags --always )
TARGET        := $(shell basename `git rev-parse --show-toplevel`)
TEST          ?= $(shell go list ./... | grep -v /vendor/)
REPOSITORY    := mattdeboer/mpp
DOCKER_IMAGE   = ${REPOSITORY}:${VERSION}

default: test build

test:
	go test -v -cover -run=$(RUN) $(TEST)

build: clean
	@go build -v -o bin/$(TARGET) ./pkg/server

release: clean
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build \
		-a -tags netgo \
		-a -installsuffix cgo \
    -ldflags "-s -X main.Version=$(VERSION) -X main.Name=$(TARGET)" \
		-o bin/$(TARGET) ./pkg/server

ca-certificates.crt:
	@-docker rm -f mpp_cacerts
	@docker run --name mpp_cacerts debian:latest bash -c 'apt-get update && apt-get install -y ca-certificates'
	@docker cp mpp_cacerts:/etc/ssl/certs/ca-certificates.crt .
	@docker rm -f mpp_cacerts

docker: ca-certificates.crt
	@echo "Building ${DOCKER_IMAGE}..."
	@docker build -t ${DOCKER_IMAGE} .

clean:
	@rm -rf bin/