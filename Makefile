.PHONY:build build-agent build-gui build-gui-macos build-all clean test dev
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
# GUI is a windowsgui binary: no console window, mygo inspector disabled.
GUI_LDFLAGS = -ldflags "-s -w -H=windowsgui -X github.com/egoist/mygo.production=1 -X main.version=$(VERSION)"


build: mod-tidy
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) \
	 $(LDFLAGS)  -o ./dist/$(BINARY_NAME).exe

build-agent:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) \
	 $(LDFLAGS) -o ./dist/wslpp-agent-linux-amd64 ./cmd/agent
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) \
	 $(LDFLAGS) -o ./dist/wslpp-agent-linux-arm64 ./cmd/agent

build-gui: mod-tidy
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -trimpath -tags mygo_noinspector \
	 $(GUI_LDFLAGS) -o ./dist/wslpp-gui.exe ./cmd/gui

# macOS packaging must run on a Mac: mygo builds a signed .app and a DMG.
build-gui-macos: mod-tidy
	bash deploy/package-macos.sh

build-all: build build-agent build-gui

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	@rm -f ./dist/$(BINARY_NAME)_*

mod-tidy:
	$(GOCMD) mod tidy

dev:
	$(GORUN) ./main.go

