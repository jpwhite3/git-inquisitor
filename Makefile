export PATH := $(shell go env GOPATH)/bin:$(PATH)

.ONESHELL:
.PHONY: clean clean-build clean-test help build test test-html test-debug lint lint-fix fmt version tag release bootstrap

.DEFAULT_GOAL := help

define PRINT_HELP_GOSCRIPT
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	re := regexp.MustCompile(`^([a-zA-Z_-]+):.*?## (.*)$$`)
	
	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) == 3 {
			target := matches[1]
			help := matches[2]
			fmt.Printf("%-20s %s\n", target, help)
		}
	}
	
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}
}
endef
export PRINT_HELP_GOSCRIPT

help: ## display this help message
	@go run -e "$$PRINT_HELP_GOSCRIPT" < $(MAKEFILE_LIST)

clean: clean-build clean-test ## remove all build, test, coverage and artifacts

clean-build: ## remove build artifacts
	rm -rf bin/ dist/
	rm -f *.out

clean-test: ## remove test and coverage artifacts
	rm -f coverage.out coverage.html
	rm -rf .test-cache

test: ## run tests quickly with coverage
	go test ./... -v -cover

test-html: ## run tests and generate HTML coverage report
	go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out -o coverage.html

test-debug: ## run tests with delve debugger
	dlv test ./...

fmt: ## format Go code
	go fmt ./...

version: ## set version based on date
	@mkdir -p internal/version
	@echo "package version\n\n// Version is the current application version\nconst Version = \"$(shell date +'%Y.%-m.%-d')\"" > internal/version/version.go
	@echo "Version set to $(shell date +'%Y.%-m.%-d')"

build: clean version ## build binary
	go build -o bin/git-inquisitor ./cmd/git-inquisitor

run: build ## run binary
	./bin/git-inquisitor

lint: ## lint Go code with golangci-lint
	golangci-lint run ./...

lint-fix: ## automatically fix linting errors where possible
	golangci-lint run --fix ./...

bootstrap: ## install development dependencies
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/go-delve/delve/cmd/dlv@latest
	go install github.com/goreleaser/goreleaser@latest

tag: ## create and push git tag with date-based version
	export VERSION=$(shell date +'%Y.%-m.%-d')
	git tag -d $(shell date +'%Y.%-m.%-d') 2>/dev/null || true
	git push origin ":refs/tags/$(shell date +'%Y.%-m.%-d')" 2>/dev/null || true
	git tag -f $(shell date +'%Y.%-m.%-d')
	git push --tags

release: clean version tag ## create github release with goreleaser
	goreleaser release --clean
