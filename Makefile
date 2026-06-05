# Version is derived from git tags (e.g. v0.2.0), falling back to the short
# commit hash, with a -dirty suffix when the working tree has changes.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
MODULE  := github.com/totalizator/sshc
LDFLAGS := -X $(MODULE)/cmd.version=$(VERSION)

BIN := sshc
ifeq ($(OS),Windows_NT)
	BIN := sshc.exe
endif

# ansisvg lays out the ANSI dumps on a fixed character grid. Two things matter for
# seamless rows:
#   * cell HEIGHT must match the glyph height, or the half-block gutter bar (▌) and
#     the box-drawing borders (│), drawn as fontsize-16 glyphs, don't fill the cell
#     and leave a hairline seam between stacked rows. 9x17 makes them tile; 9x18/19
#     reopen the seam.
#   * rasterize.ps1 must bake the SVG to PNG at its NATURAL size (1152x578 here:
#     128*9 by 34*17). Any other window size makes Chrome rescale by a non-integer
#     factor and reopens a ~1px gap. (GitHub sanitizes SVG <style>, so we ship PNG.)
ANSISVG := go run github.com/wader/ansisvg@latest --grid --charboxsize 9x17 --fontsize 16 --fontname Consolas,Menlo,monospace

.PHONY: build install test vet fmt clean version shots preview export

build: ## Build a version-stamped binary
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

install: ## Install a version-stamped binary into GOBIN
	go install -ldflags "$(LDFLAGS)" .

test: ## Run tests
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files
	gofmt -w .

version: ## Print the version that would be stamped
	@echo $(VERSION)

clean: ## Remove built binaries
	rm -f sshc sshc.exe

shots: ## Regenerate demo/*.svg screenshots from the live TUI (needs a POSIX shell)
	SSHC_SHOT=1 go test ./tui -run TestGenerateShots
	$(ANSISVG) < demo/_ansi/01-list.ansi   > demo/01-list.svg
	$(ANSISVG) < demo/_ansi/02-search.ansi > demo/02-search.svg
	$(ANSISVG) < demo/_ansi/03-detail.ansi > demo/03-detail.svg

preview: ## Build the looping demo/preview.webp from the rasterized PNGs (needs Python+Pillow)
	python demo/animate.py

export: ## Regenerate the scrubbed public GitHub export repo (see tools/export.sh)
	bash tools/export.sh
