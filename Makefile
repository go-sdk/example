MAKEFLAGS       += --no-print-directory

VERSION         ?= v1.0.0
BIN_DIR         ?= bin

GO_LDFLAGS_PART := -s -w
GO_LDFLAGS_PART += -X "github.com/go-sdk/core/osx.iVersion=$(VERSION)"
GO_LDFLAGS      := -ldflags '$(GO_LDFLAGS_PART)'

PROTOC_GEN_GO_VERSION           ?= v1.36.12 # https://github.com/protocolbuffers/protobuf-go
PROTOC_GEN_GO_GRPC_VERSION      ?= v1.6.2   # https://github.com/grpc/grpc-go
PROTOC_GEN_GRPC_GATEWAY_VERSION ?= v2.30.0  # https://github.com/grpc-ecosystem/grpc-gateway
PROTOC_GEN_OPENAPIV2_VERSION    ?= v2.30.0  # https://github.com/grpc-ecosystem/grpc-gateway（与 protoc-gen-grpc-gateway 同仓库同版本）
PROTOC_GEN_GO_JSON_VERSION      ?= v1.1.3   # https://github.com/protoc-contrib/protoc-gen-go-json

.PHONY: tidy
tidy:					##@ Tidy go.mod and go.sum.
	@go mod tidy

.PHONY: prepare
prepare:				##@ Install buf local plugins.
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@$(PROTOC_GEN_GRPC_GATEWAY_VERSION)
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@$(PROTOC_GEN_OPENAPIV2_VERSION)
	@go install github.com/protoc-contrib/protoc-gen-go-json/cmd/protoc-gen-go-json@$(PROTOC_GEN_GO_JSON_VERSION)

.PHONY: generate
generate:				##@ Lint & Generate proto files.
	@if command -v buf >/dev/null 2>&1; then \
		buf lint && \
		tmp_dir=$$(mktemp -d); \
		trap 'rm -rf "$$tmp_dir"' EXIT; \
		cp buf.gen.yaml "$$tmp_dir/buf.gen.yaml"; \
		sed -i.bak -e "s|out: \.$$|out: $$tmp_dir/generated|" -e "s|out: openapi$$|out: $$tmp_dir/generated/openapi|" "$$tmp_dir/buf.gen.yaml"; \
		rm -f "$$tmp_dir/buf.gen.yaml.bak"; \
		buf generate --template "$$tmp_dir/buf.gen.yaml"; \
		test -d "$$tmp_dir/generated/gen"; \
		test -f "$$tmp_dir/generated/openapi/openapi.swagger.yaml"; \
		rm -rf gen openapi; \
		mv "$$tmp_dir/generated/gen" gen; \
		mv "$$tmp_dir/generated/openapi" openapi; \
		echo "done."; \
	else \
		echo "buf is not installed. Please install it from https://github.com/bufbuild/buf"; \
		exit 1; \
	fi

.PHONY: lint
lint: tidy				##@ Lint all packages.
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout 5m && \
		echo "done."; \
	else \
		echo "golangci-lint is not installed. Please install it from https://github.com/golangci/golangci-lint"; \
		exit 1; \
	fi

.PHONY: build
build:					##@ Build cmd/app into bin.
	@mkdir -p $(BIN_DIR)
	@go build $(GO_LDFLAGS) -o $(BIN_DIR)/app ./cmd/app

.PHONY: run
run: build				##@ Run app.
	@CONFIG_PATH=config.yaml $(BIN_DIR)/app $(ARGS)


.PHONY: help
help:					##@ (Default) Show help.
	@printf "\nUsage: make <command>\n"
	@grep -F -h "##@" $(MAKEFILE_LIST) | grep -F -v grep -F | sed -e 's/\\$$//' | awk 'BEGIN {FS = ":*[[:space:]]*##@[[:space:]]*"}; \
	{ \
		if($$2 == "") \
			pass; \
		else if($$0 ~ /^#/) \
			printf "\n%s\n", $$2; \
		else if($$1 == "") \
			printf "     %-20s%s\n", "", $$2; \
		else \
			printf "\n    \033[34m%-20s\033[0m %s\n", $$1, $$2; \
	}'
	@printf "\n"

.DEFAULT_GOAL := help
