BIN     := dustit
PREFIX  ?= /usr/local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/iamolayemi/dustit/cmd.version=$(VERSION)

.PHONY: build test lint install snapshot clean docs demo menubar

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .

test:
	go vet ./...
	go test -race ./internal/... ./cmd/... && go test -count=1 ./e2e/

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

# Builds the menu bar app into dist/DustIt.app (needs Xcode or the Command Line Tools).
APP := dist/DustIt.app
menubar:
	cd macos/DustIt && swift build -c release
	rm -rf "$(APP)" && mkdir -p "$(APP)/Contents/MacOS"
	cp macos/DustIt/.build/release/DustIt "$(APP)/Contents/MacOS/"
	cp macos/DustIt/Info.plist "$(APP)/Contents/"
	mkdir -p "$(APP)/Contents/Resources" && rm -rf dist/AppIcon.iconset
	macos/DustIt/.build/release/DustIt --iconset dist/AppIcon.iconset >/dev/null
	iconutil -c icns dist/AppIcon.iconset -o "$(APP)/Contents/Resources/AppIcon.icns"
	codesign --force --sign - "$(APP)"   # ad-hoc signature so macOS will run the local build
	@echo "Built $(APP) — move it to /Applications and open it"
