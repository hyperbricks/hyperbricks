#!/bin/bash
echo "Apache Benchmarking"
ab -n 10000 -c 150 http://192.168.2.11:8080/
