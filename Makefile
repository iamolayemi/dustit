BIN     := dustit
PREFIX  ?= /usr/local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/iamolayemi/dustit/cmd.version=$(patsubst v%,%,$(VERSION)) \
	-X github.com/iamolayemi/dustit/cmd.commit=$(shell git rev-parse --short HEAD 2>/dev/null) \
	-X github.com/iamolayemi/dustit/cmd.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: build test lint install snapshot clean docs demo menubar menubar-zip

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
# It's universal (Apple Silicon + Intel) and carries its own universal copy of the dustit CLI,
# so it works even when the CLI isn't installed.
APP := dist/DustIt.app
SWIFT_ARCHS := --arch arm64 --arch x86_64
APP_VERSION := $(patsubst v%,%,$(VERSION))
menubar:
	cd macos/DustIt && swift build -c release $(SWIFT_ARCHS)
	rm -rf "$(APP)" && mkdir -p "$(APP)/Contents/MacOS" "$(APP)/Contents/Resources"
	cp "$$(cd macos/DustIt && swift build -c release $(SWIFT_ARCHS) --show-bin-path)/DustIt" "$(APP)/Contents/MacOS/"
	cp macos/DustIt/Info.plist "$(APP)/Contents/"
	/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $(APP_VERSION)" "$(APP)/Contents/Info.plist" || true
	rm -rf dist/AppIcon.iconset && "$(APP)/Contents/MacOS/DustIt" --iconset dist/AppIcon.iconset >/dev/null
	iconutil -c icns dist/AppIcon.iconset -o "$(APP)/Contents/Resources/AppIcon.icns"
	GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/dustit-arm64 .
	GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/dustit-amd64 .
	lipo -create -output "$(APP)/Contents/Resources/dustit" dist/dustit-arm64 dist/dustit-amd64
	rm -f dist/dustit-arm64 dist/dustit-amd64
	codesign --force --deep --sign - "$(APP)"   # ad-hoc signature so macOS will run it
	@echo "Built $(APP) $(APP_VERSION) — move it to /Applications and open it"

# Zips the app for a release: dist/DustIt-<version>-macOS.zip
menubar-zip: menubar
	rm -f "dist/DustIt-$(APP_VERSION)-macOS.zip"
	ditto -c -k --sequesterRsrc --keepParent "$(APP)" "dist/DustIt-$(APP_VERSION)-macOS.zip"
	@echo "Zipped dist/DustIt-$(APP_VERSION)-macOS.zip"
