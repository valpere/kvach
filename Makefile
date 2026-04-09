GO ?= go
BIN_DIR ?= bin
BIN_NAME ?= kvach

.PHONY: help build build-cli run test vet fmt fmt-check tidy check ci clean

help:
	@printf "Targets:\n"
	@printf "  make build       - Build all Go packages\n"
	@printf "  make build-cli   - Build kvach binary to ./$(BIN_DIR)/$(BIN_NAME)\n"
	@printf "  make run         - Run kvach CLI from source\n"
	@printf "  make test        - Run all tests\n"
	@printf "  make vet         - Run go vet\n"
	@printf "  make fmt         - Format all Go files with gofmt\n"
	@printf "  make fmt-check   - Fail if any Go file is not gofmt-formatted\n"
	@printf "  make tidy        - Run go mod tidy\n"
	@printf "  make check       - Run fmt, build, vet, and test\n"
	@printf "  make ci          - Run CI-equivalent checks (fmt-check, build, vet, test)\n"
	@printf "  make clean       - Remove local build artifacts\n"

build:
	$(GO) build ./...

build-cli:
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -o "$(BIN_DIR)/$(BIN_NAME)" ./cmd/kvach

run:
	$(GO) run ./cmd/kvach

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		printf "Unformatted files:\n%s\n" "$$files"; \
		false; \
	fi

tidy:
	$(GO) mod tidy

check: fmt build vet test

ci: fmt-check build vet test

clean:
	rm -rf "$(BIN_DIR)"
