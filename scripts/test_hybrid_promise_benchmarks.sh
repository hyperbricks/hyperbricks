#!/bin/bash
set -euo pipefail

echo "HyperBricks hybrid promise benchmark"
echo
echo "Measures speed, flexibility, source lines, and setup cost for a small server-rendered HTMX-style HyperBricks module."
echo "The verbose test output explains what the numbers mean; the benchmark rows provide ns/op and allocation data."
echo

go test -v -run TestHybridPromiseMetricsAreQuantifiable -bench '^BenchmarkHybridPromise' -benchmem ./test/docs
