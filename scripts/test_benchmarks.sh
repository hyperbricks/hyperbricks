#!/bin/bash
echo "Benchmarking for channel implementation"
go test -bench ^BenchmarkConcurrentRecursiveRender_NestedItems$ -benchmem -race ./test/main
echo "Benchmarking for mutex implementation"
go test -bench ^BenchmarkConcurrentRecursiveRenderMutex_NestedItems$ -benchmem -race ./test/main
