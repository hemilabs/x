# Copyright © 2019 Binance
# Copyright (c) 2026 Hemi Labs, Inc.
# Use of this source code is governed by the MIT License,
# which can be found in the LICENSE file.

PROJECTPATH = $(abspath $(dir $(realpath $(firstword $(MAKEFILE_LIST)))))

export GOBIN=$(PROJECTPATH)/bin
export GOCACHE=$(PROJECTPATH)/.gocache
export GOPKG=$(PROJECTPATH)/pkg

# renovate: datasource=github-releases depName=golangci/golangci-lint versioning=semver
GOLANGCI_LINT_VERSION="v2.7.2"
# renovate: datasource=github-releases depName=joshuasing/golicenser versioning=semver
GOLICENSER_VERSION="v0.3.1"
# renovate: datasource=github-releases depName=mvdan/gofumpt versioning=semver
GOFUMPT_VERSION="v0.9.2"

.PHONY: all clean deps go-deps build lint lint-deps tidy race test vulncheck vulncheck-deps

all: tidy build lint test

clean:
	rm -rf $(GOBIN) $(GOCACHE) $(GOPKG)

deps: lint-deps vulncheck-deps go-deps

go-deps:
	go mod download
	go mod tidy
	go mod verify

build:
	go build ./...

define LICENSE_HEADER
Copyright (c) {{.year}} {{.author}}
Use of this source code is governed by the MIT License,
which can be found in the LICENSE file.
endef
export LICENSE_HEADER

lint:
	$(shell go env GOPATH)/bin/golangci-lint fmt ./...
	$(shell go env GOPATH)/bin/golangci-lint run --fix ./...

lint-deps:
	@echo "Installing with $(shell go env GOVERSION)"
	GOBIN=$(shell go env GOPATH)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	GOBIN=$(shell go env GOPATH)/bin go install github.com/joshuasing/golicenser/cmd/golicenser@$(GOLICENSER_VERSION)
	GOBIN=$(shell go env GOPATH)/bin go install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)

tidy:
	go mod tidy

race:
	go test -v -race -timeout 60m ./...

test:
	go test -timeout 30m -coverprofile=$(PROJECTPATH)/coverage.out -covermode=atomic ./...

vulncheck:
	$(shell go env GOPATH)/bin/govulncheck ./...

vulncheck-deps:
	GOBIN=$(shell go env GOPATH)/bin go install golang.org/x/vuln/cmd/govulncheck@latest
