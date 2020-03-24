export GO111MODULE=on

TIMESTAMP := $(shell date '+%m%d%H%M%Y.%S')
RELEASE_TAG   ?= $(TIMESTAMP)

# Default Go linker flags.
GO_LDFLAGS ?= -ldflags="-s -w -X main.Version=${RELEASE_TAG}"

# Binary name.
REPLAY := ./bin/replay-zero
REPLAYOSX := ./bin/replay-zero-osx
REPLAYWIN := ./bin/replay-zero.exe

.PHONY: all
all: clean lint $(REPLAY) $(REPLAYOSX) $(REPLAYWIN) test

$(REPLAY):
	GOOS=linux go build -mod=vendor $(GO_LDFLAGS) $(BUILDARGS) -o $@ .

$(REPLAYOSX):
	GOOS=darwin GOARCH=amd64 go build -mod=vendor $(GO_LDFLAGS) $(BUILDARGS) -o $@ .

$(REPLAYWIN):
	GOOS=windows GOARCH=386  go build -mod=vendor $(GO_LDFLAGS) $(BUILDARGS) -o $@ .

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor

.PHONY: lint
lint:
	@ golangci-lint run --fast

.PHONY: test
test:
	go test -mod=vendor -timeout=30s $(TESTARGS) ./...

.PHONY: cover
cover:
	@$(MAKE) test TESTARGS="-coverprofile=coverage.out"
	@go tool cover -html=coverage.out
	@rm -f coverage.out

.PHONY: clean
clean:
	@rm -rf ./bin

.PHONY: package
package: all
	zip -j bin/replay-zero.zip $(REPLAY)
	zip -j bin/replay-zero-osx.zip $(REPLAYOSX)
	zip -j bin/replay-zero-win.zip $(REPLAYWIN)
	shasum -a 256 bin/replay-zero.zip > bin/replay-zero.sha256
	shasum -a 256 bin/replay-zero-osx.zip > bin/replay-zero-osx.sha256
	shasum -a 256 bin/replay-zero-win.zip > bin/replay-zero-win.sha256
