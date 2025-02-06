VERSION=`git describe --tags`
BUILD=`date +%FT%T%z`
HASH=`git rev-parse --short HEAD`

LDFLAGS=-ldflags "-w -s -X main.version=${VERSION} -X main.buildDate=${BUILD} -X main.gitCommit=${HASH}"

.PHONY: all clean

all: test-and-build

build:
	go build ${LDFLAGS} -o frodo

test-and-build:
	go test ./...
	swag init --parseDependency -g frodo.go
	go build ${LDFLAGS} -o frodo

swagger:
	@echo "generating swagger docs"
	@go install github.com/swaggo/swag/cmd/swag@latest
	@swag init --parseDependency -g frodo.go

clean:
	@rm -rf docs/*
	@rm frodo

test:
	go test ./...
