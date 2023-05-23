GOROOT ?= $(shell go env GOROOT)
SOURCES := $(shell find . -name '*.go')

.PHONY: wasm
wasm: public_html/wasm.wasm public_html/* public_html/data/*

public_html/wasm_exec.js: $(GOROOT)/misc/wasm/wasm_exec.js
	cp $(GOROOT)/misc/wasm/wasm_exec.js public_html/

public_html/wasm.wasm: Makefile $(SOURCES)
	GOOS=js GOARCH=wasm go build -o public_html/wasm.wasm ./cmd/wasm

public_html/data/core.json: constraints/core.json
	cp constraints/core.json public_html/data/

public_html/data/tt.json: constraints/tt.json
	cp constraints/tt.json public_html/data/

