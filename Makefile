GO_PKGS := $(shell go list ./... | grep -v /vendor)
BIN_DIR := $(GOPATH)/bin
GOMETALINTER := $(BIN_DIR)/gometalinter
BINARY := mdbload
PLATFORMS := windows linux darwin
BUILD_TIME := $(shell date -u +%Y%m%d.%H%M%S)
VERSION := $(shell git describe --always --long --dirty)
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
release: windows linux darwin docker-publish ## build releases

.PHONY: docker
docker:
	docker build --pull --build-arg GIT_SHA=$(GIT_SHA) --build-arg VERSION=$(VERSION) --build-arg BUILD_TIME=$(BUILD_TIME) -t quay.io/scbunn/mdbload:$(VERSION) .
	docker image tag quay.io/scbunn/mdbload:$(VERSION) quay.io/scbunn/mdbload:latest

.PHONY: docker-publish
docker-publish: docker
	docker push quay.io/scbunn/mdbload:$(VERSION)
	docker push quay.io/scbunn/mdbload:latest

clean:  ## clean project
	@echo "cleaning project"
	@go clean
	@rm -rf release

build:  ## build project for current platform
	go build $(LDFLAGS) -race
