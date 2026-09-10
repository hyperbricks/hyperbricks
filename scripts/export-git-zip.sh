#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

git archive --format=zip --prefix=hyperbricks/ --output=../hyperbricks-4e96f94.zip HEAD

# Build hyperbricks cms for linux
echo "HyperBrics export complete!"
