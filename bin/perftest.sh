#!/usr/bin/env bash
set -e
SCRIPTPATH=$( cd "$(dirname "$0")" ; pwd -P )

ROOT=$SCRIPTPATH/..
cd $ROOT


echo "Perf test"
DIRS="pkg/api pkg/expr pkg/il/interpreter"
cd $ROOT
for pkgdir in ${DIRS}; do
    bazel run ${pkgdir}:go_default_test -- -test.run='^$' -test.bench=. -test.benchmem
done
