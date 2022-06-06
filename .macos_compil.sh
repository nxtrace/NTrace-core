#!/usr/bin/env bash

set -e

BUILD_VERSION=$1
BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
COMMIT_SHA1=""

go build -trimpath -ldflags "-X 'github.com/xgadget-lab/nexttrace/printer.version=${BUILD_VERSION}' \
                             -X 'github.com/xgadget-lab/nexttrace/printer.buildDate=${BUILD_DATE}' \
                             -X 'github.com/xgadget-lab/nexttrace/printer.commitID=${COMMIT_SHA1}'\
                             -w -s"