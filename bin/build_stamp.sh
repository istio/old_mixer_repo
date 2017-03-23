#!/bin/bash

# This must be kept in sync with cmd/server/version.go
echo "BuildID $(date +%F)-$(git rev-parse --short HEAD)"
