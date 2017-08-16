# Copyright 2017 IBM Corporation
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

.DEFAULT_GOAL	:= build

#------------------------------------------------------------------------------
# Variables
#------------------------------------------------------------------------------

SHELL      := /bin/bash

GOFILES		= `find . -type f -name '*.go' -not -path "./vendor/*"`
GODIRS		= `go list -f '{{.Dir}}' ./... | grep -vxFf <(go list -f '{{.Dir}}' ./vendor/...)`
GOPKGS		= `go list ./... | grep -vxFf <(go list ./vendor/...)`

DOCKER_REGISTRY_HOST    := artifactory.haifa.ibm.com:5020

IMAGE_NAME	:= clearharbor/auth-server:latest
DOCKERFILE	:= build/Dockerfile
BINDIR		:= ./bin

# build flags to create a statically linked binary, using native go networking
BUILDFLAGS	:= -i -tags "netgo"
LDFLAGS         := -linkmode external -extldflags -static -s -w
GOBUILD		:= export CGO_ENABLED=0; go build

#--------------
#-- high-level
#--------------
.PHONY: verify precommit

# to be run by CI to verify validity of code changes
verify: check build test

# to be run by developer before checking-in code changes
precommit: format verify

#------------------
#-- dependencies
#------------------
.PHONY: depend.update depend.install

depend.update: tools.glide
	@echo "--> updating dependencies from glide.yaml"
	@glide update --strip-vendor

depend.install:	tools.glide
	@echo "--> installing dependencies from glide.lock "
	@glide install --strip-vendor

#---------
#-- build
#---------
.PHONY: build compile clean

compile:
	@echo "--> compiling packages"
	@$(GOBUILD) $(GOPKGS)

build:
	@echo "--> building clearharbor binary"
	@$(GOBUILD) $(BUILDFLAGS) -ldflags '$(LDFLAGS)' -o $(BINDIR)/clearharbor .

clean:
	@echo "--> cleaning compiled objects and binaries"
	@go clean -i $(GOPKGS)
	@rm -rf $(BINDIR)/*

#--------
#-- test
#--------
.PHONY: test test.long

test:
	@echo "--> running unit tests, excluding long tests"
	@go test -v $(GOPKGS) -short

test.long:
	@echo "--> running unit tests, including long tests"
	@go test -v $(GOPKGS)

#---------------
#-- checks
#---------------
.PHONY: check format format.check vet lint

check: format.check vet lint

format: tools.goimports
	@echo "--> formatting code with 'goimports' tool"
	@goimports -w -l $(GOFILES)

format.check: tools.goimports
	@echo "--> checking code formatting with 'goimports' tool"
	@goimports -l $(GOFILES) | sed -e "s/^/\?\t/" | tee >(test -z)

vet: tools.govet
	@echo "--> checking code correctness with 'go vet' tool"
	@go vet $(GOPKGS)

lint: tools.golint
	@echo "--> checking code style with 'golint' tool"
	@echo $(GODIRS) | xargs -n 1 golint

#---------------
#-- dockerize
#---------------
.PHONY: docker.build docker.push

docker.build:
	@echo "--> building clearharbor docker image"
	@docker build -t $(IMAGE_NAME) -f $(DOCKERFILE) .
		
docker.push:
	@echo "--> pushing clearharbor docker image"
	@docker tag $(IMAGE_NAME) $(DOCKER_REGISTRY_HOST)/$(IMAGE_NAME)
	@docker push $(DOCKER_REGISTRY_HOST)/$(IMAGE_NAME)

#---------------
#-- tools
#---------------
.PHONY: tools tools.goimports tools.golint tools.govet tools.glide

tools: tools.goimports tools.golint tools.govet tools.glide

tools.goimports:
	@command -v goimports >/dev/null ; if [ $$? -ne 0 ]; then \
		echo "--> installing goimports"; \
		go get golang.org/x/tools/cmd/goimports; \
    fi

tools.govet:
	@go tool vet 2>/dev/null ; if [ $$? -eq 3 ]; then \
		echo "--> installing govet"; \
		go get golang.org/x/tools/cmd/vet; \
	fi

tools.golint:
	@command -v golint >/dev/null ; if [ $$? -ne 0 ]; then \
		echo "--> installing golint"; \
		go get github.com/golang/lint/golint; \
    fi

tools.glide:
	@command -v glide >/dev/null ; if [ $$? -ne 0 ]; then \
		echo "--> installing glide"; \
		mkdir -p /tmp/glide-0.12.3-linux-amd64; \
		wget -qO- https://github.com/Masterminds/glide/releases/download/v0.12.3/glide-v0.12.3-linux-amd64.tar.gz | tar xz -C /tmp/glide-0.12.3-linux-amd64; \
		cp /tmp/glide-0.12.3-linux-amd64/linux-amd64/glide ~/bin/; \
    fi
