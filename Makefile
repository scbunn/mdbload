GO_PKGS := $(shell go list ./... | grep -v /vendor)
BIN_DIR := $(GOPATH)/bin
GOMETALINTER := $(BIN_DIR)/gometalinter
BINARY := mdbload
PLATFORMS := windows linux darwin
VERSION ?= v0.0.0.Development
BUILD_TIME := $(shell date -u +%Y%m%d.%H%M%S)
GIT_SHA := $(shell git describe --always --long --dirty)
GIT_SHA := $(shell git rev-parse HEAD)
LDFLAGS := -ldflags "-w -s -X main.BUILD_DATE=$(BUILD_TIME) -X main.GIT_SHA=$(GIT_SHA) -X main.VERSION=$(VERSION)"
os = $(word 1, $@)

$(GOMETALINTER):
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install &> /dev/null

.PHONY: test
test:  ## Execute test suite
	go test $(PKGS)

.PHONY: lint
lint: $(GOMETALINTER)  ## go lint project
	gometalinter ./... --vendor

.PHONY: $(PLATFORMS)
$(PLATFORMS):
	mkdir -p release
	GOOS=$(os) GOARCH=amd64 go build $(LDFLAGS) -o release/$(BINARY)-$(VERSION)-$(os)-amd64

.PHONY: release
release: windows linux darwin  ## build releases

clean:  ## clean project
	@echo "cleaning project"
	@go clean
	@rm -rf release

build:  ## build project for current platform
	go build $(LDFLAGS) -race
