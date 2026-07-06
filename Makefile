GO ?= go
GOLANGCI_LINT ?= golangci-lint
GOWORK ?= off
GO_TEST_FLAGS ?=
RACE_PACKAGES ?= ./threadsafemap ./yathreadsafeset

SUPPORTED_PACKAGES := $(shell GOWORK=$(GOWORK) $(GO) list ./... | grep -v '\.dev')
LINT_PACKAGES := $(patsubst github.com/YaCodeDev/GoYaCodeDevUtils/%,./%,$(SUPPORTED_PACKAGES))

.PHONY: all tidy format lint test test-race packages

all: tidy format lint test

tidy:
	GOWORK=$(GOWORK) $(GO) mod tidy

format:
	GOWORK=$(GOWORK) $(GOLANGCI_LINT) fmt

lint:
	GOWORK=$(GOWORK) $(GOLANGCI_LINT) run $(LINT_PACKAGES)

test:
	GOWORK=$(GOWORK) $(GO) test $(GO_TEST_FLAGS) $(SUPPORTED_PACKAGES)

test-race:
	GOWORK=$(GOWORK) $(GO) test -race $(RACE_PACKAGES)

packages:
	@printf '%s\n' $(SUPPORTED_PACKAGES)
