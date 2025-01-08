#!/bin/zsh

go run -ldflags "\
    -X 'main.Version=$(cat version.md | tr -d \n)' \
    -X 'main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
./cmd/hyperbricks-docs/main.go