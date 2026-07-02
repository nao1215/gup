.PHONY: build test e2e clean vet fmt chkfmt changelog update-tools help coverage-tree

APP         = gup
VERSION     = $(shell git describe --tags --abbrev=0)
GO          = go
GO_BUILD    = $(GO) build
GO_FORMAT   = $(GO) fmt
GO_GET      = $(GO) get
GOFMT       = gofmt
GO_LIST     = $(GO) list
GO_TEST     = $(GO) test -v
GO_TOOL     = $(GO) tool
GO_VET      = $(GO) vet
GO_DEP      = $(GO) mod
GOOS        = ""
GOARCH      = ""
GO_PKGROOT  = ./...
GO_PACKAGES = $(shell $(GO_LIST) $(GO_PKGROOT))
GO_LDFLAGS  = -ldflags '-X github.com/nao1215/gup/internal/cmdinfo.Version=${VERSION}'

build:  ## Build binary
	env GO111MODULE=on GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) $(GO_LDFLAGS) -o $(APP) main.go

clean: ## Clean project
	-rm -rf $(APP) cover.out cover.html


test: ## Start test
	env GOOS=$(GOOS) $(GO_TEST) -cover $(GO_PKGROOT) -coverprofile=cover.out
	$(GO_TOOL) cover -html=cover.out -o cover.html

e2e: ## Run offline end-to-end tests against the real CLI (requires atago)
	./e2e/run.sh

vet: ## Start go vet
	$(GO_VET) $(GO_PACKAGES)

fmt: ## Format go source code 
	$(GO_FORMAT) $(GO_PKGROOT)

coverage-tree: test ## Generate coverage tree
	$(GO_TOOL) go-cover-treemap -statements -coverprofile cover.out > doc/img/cover-tree.svg

update-tools: ## Add or update tool dependencies in go.mod/go.sum
	$(GO_GET) -tool github.com/nikolaydubina/go-cover-treemap@latest

.DEFAULT_GOAL := help
help:  
	@grep -E '^[0-9a-zA-Z_-]+[[:blank:]]*:.*?## .*$$' $(MAKEFILE_LIST) | sort \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[1;32m%-15s\033[0m %s\n", $$1, $$2}'
