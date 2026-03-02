# The commands used in this Makefile expect to be interpreted by bash.
SHELL := /bin/bash

# Build the code
VERSION_LDFLAGS=\
 -X "github.com/calypr/forge/version.BuildDate=$(shell date)" \
 -X "github.com/calypr/forge/version.GitCommit=$(shell git rev-parse --short HEAD)" \
 -X "github.com/calypr/forge/version.GitBranch=$(shell git symbolic-ref -q --short HEAD)" \
 -X "github.com/calypr/forge/version.Version=$(shell git describe --tags --always)"

# Build forge
build:
	@go build -ldflags '$(VERSION_LDFLAGS)' -o forge .

# Install forge
install:
	@go install -ldflags '$(VERSION_LDFLAGS)' .

.PHONY: build install
