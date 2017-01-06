#!/bin/bash

buildifier -showlog -mode=check $(find . -name BUILD -type f)

# TODO: Enable this once more of mixer is connected and we don't
# have dead code on purpose
# codecoroner funcs ./...
# codecoroner idents ./...

gometalinter --deadline=30s --disable-all\
	--enable=aligncheck\
	--enable=deadcode\
	--enable=errcheck\
	--enable=gas\
	--enable=goconst\
	--enable=gofmt\
	--enable=gosimple\
	--enable=ineffassign\
	--enable=interfacer\
	--enable=lll --line-length=160\
	--enable=misspell\
	--enable=staticcheck\
	--enable=structcheck\
	--enable=unconvert\
	--enable=unused\
	--enable=varcheck\
	--enable=vetshadow\
	./...

# These generate warnings which we should fix, and then should enable the linters
# --enable=dupl
# --enable=gocyclo
# --enable=golint
#
# These don't seem interesting
# --enable=goimports
# --enable=gotype
