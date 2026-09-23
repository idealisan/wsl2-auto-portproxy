.PHONY:build build-agent build-all clean test dev
# Go parameters
GOCMD=go
GORUN=$(GOCMD) run
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

# name
BINARY_NAME=wslpp

# git version
VERSION := $(shell git describe --always --tags  |sed -e "s/^v//")

# LDFLAGS
LDFLAGS = -ldflags "-s -w -X main.version=$(VERSION)"


build: mod-tidy
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) \
	 $(LDFLAGS)  -o ./dist/$(BINARY_NAME).exe

build-agent:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) \
	 $(LDFLAGS) -o ./dist/wslpp-agent-linux-amd64 ./cmd/agent
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) \
	 $(LDFLAGS) -o ./dist/wslpp-agent-linux-arm64 ./cmd/agent

build-all: build build-agent

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	@rm -f ./dist/$(BINARY_NAME)_*

mod-tidy:
	$(GOCMD) mod tidy

dev:
	$(GORUN) ./main.go

