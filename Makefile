VERSION ?= dev
LDFLAGS := -X github.com/dimando/reader/converter/internal/buildinfo.Version=$(VERSION)

.PHONY: build test gui-dev gui-build gui-package lexicons embalign-venv

build: ## CLI binary (./convert)
	go build -ldflags "$(LDFLAGS)" -o convert ./cmd/convert

test:
	go test ./...

gui-dev: ## hot-reload GUI development shell
	cd cmd/convert-gui && wails3 dev

gui-build: ## production GUI binary (cmd/convert-gui/bin/tbook-converter)
	cd cmd/convert-gui && wails3 task build VERSION=$(VERSION)

gui-package: ## platform packages (AppImage/deb/rpm on Linux, .app on macOS, NSIS on Windows)
	cd cmd/convert-gui && wails3 task package VERSION=$(VERSION)

lexicons: ## dev convenience: OPUS dictionaries for lexcheck
	tools/fetch-lexicons.sh

embalign-venv: ## dev convenience: local LaBSE aligner venv
	tools/embalign-setup.sh
