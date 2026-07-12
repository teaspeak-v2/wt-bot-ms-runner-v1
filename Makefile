GO ?= go
GOLINT ?= $(GO) run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
APP_PKG ?= ./cmd/server

.PHONY: run build test vet lint tidy docker

run:
	$(GO) run $(APP_PKG)

build:
	$(GO) build ./...

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

lint:
	$(GOLINT) run ./...

tidy:
	$(GO) mod tidy

docker:
	docker build -f deployments/Dockerfile -t wt-bot-ms-runner-v1:latest .
