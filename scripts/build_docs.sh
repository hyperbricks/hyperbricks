#!/bin/zsh

#go run -ldflags "\
#    -X 'main.Version=$(cat version.md | tr -d \n)' \
#    -X 'main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
#./cmd/hyperbricks-docs/main.go

#go test  -args -version="$(cat version.md | tr -d \n)" -buildtime="$(date -u +%Y-%m-%dT%H:%M:%SZ)" ./test/docs/documentation_source_test.go -v > ./test/docs/documentation_test_results.txt

go test ./test/docs/documentation_source_test.go -v \
-args -version="$(cat version.md | tr -d \n)" \
        -buildtime="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  > ./test/docs/documentation_test_results.txt