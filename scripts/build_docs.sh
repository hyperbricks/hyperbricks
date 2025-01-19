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
cp ./docs/hyperbricks-reference-$(cat version.md | tr -d \n).md ./README.md

matches=$(grep -iF "PASS:" ./test/docs/documentation_test_results.txt);

if [ -z "$matches" ]; then
    pass_num_matches=0;
else
    pass_num_matches=$(echo "$matches" | wc -l);
fi
echo "$matches"

matches=$(grep -iF "FAIL:" ./test/docs/documentation_test_results.txt);

if [ -z "$matches" ]; then
    num_matches=0;
else
    num_matches=$(echo "$matches" | wc -l);
fi
echo "$matches"
echo "\n${pass_num_matches} tests passing";
echo "${num_matches} tests failing";


matches=$(grep -iF ": Test_TestAndDocumentationRender" ./test/docs/documentation_test_results.txt);

if [ -z "$matches" ]; then
    total_num_matches=0;
else
    total_num_matches=$(echo "$matches" | wc -l);
fi

echo "${total_num_matches} tests in total \n\n";