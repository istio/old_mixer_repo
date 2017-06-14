#!/usr/bin/env bash
set -e
DIRS="pkg/modelgen pkg/proc_interface_gen"

for pkgdir in ${DIRS}; do
    pushd ${pkgdir} > /dev/null; \
    go test
    popd > /dev/null;
done
