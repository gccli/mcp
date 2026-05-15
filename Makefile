.PHONY: all build clean test fmt vet lint run-ssh help

BINARY_NAME := mcp
BUILD_DIR := /usr/local/bin
GO := go
GOFLAGS := -v

# 默认目标
all: build

## build: 编译项目
build:
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

## clean: 清理构建产物
clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)

## test: 运行所有测试
test:
	$(GO) test $(GOFLAGS) ./...

## test-race: 运行测试并检测竞态条件
test-race:
	$(GO) test -race $(GOFLAGS) ./...

## fmt: 格式化 Go 代码
fmt:
	$(GO) fmt ./...

## vet: 运行 go vet 静态分析
vet:
	$(GO) vet ./...

## lint: 运行 fmt + vet
lint: fmt vet

## run-ssh: 编译并运行 SSH MCP Server
run-ssh: build
	$(BUILD_DIR)/$(BINARY_NAME) ssh

## inspect: 使用 MCP Inspector 调试 SSH Server
inspect: build
	npx @modelcontextprotocol/inspector $(BUILD_DIR)/$(BINARY_NAME) ssh

## deps: 下载并整理依赖
deps:
	$(GO) mod download
	$(GO) mod tidy

## help: 显示本帮助信息
help:
	@echo "可用的 make 目标："
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## /  /'
