BIN     := macclean
PREFIX  ?= /usr/local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/iamolayemi/macclean/cmd.version=$(VERSION)

.PHONY: build test lint install snapshot clean docs demo

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .

test:
	go vet ./...
	go test -race ./...

lint:
	golangci-lint run

install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BIN) $(PREFIX)/bin/$(BIN)

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf $(BIN) dist generated

docs:
	go run . gen generated
	@echo "man pages in generated/man, completions in generated/completions"

# Records docs/demo.gif from docs/demo.tape (needs https://github.com/charmbracelet/vhs).
demo: build
	vhs docs/demo.tape
