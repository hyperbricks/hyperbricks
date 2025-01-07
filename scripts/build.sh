#!/bin/zsh

# Exit immediately if a command exits with a non-zero status
set -e

# Navigate to the project root directory (assuming `scripts` is a subdirectory of the root)
cd "$(dirname "$0")/.."

# Enable SQLite3 interface
set CGO_ENABLED=1
go env -w CGO_ENABLED=1

echo "Current directory: $(pwd)"
echo "Building plugins"

# Build plugins
go build -buildmode=plugin -o ./bin/plugins/LoremIpsumPlugin.so ./plugins/loremipsum/lorem_ipsum_plugin.go
go build -buildmode=plugin -o ./bin/plugins/myplugin.so ./plugins/myplugin/my_plugin.go

# Build hyperbricks cms
go build -v -o ./bin/hyperbricks ./cmd/hyperbricks

echo "Build complete!\n\n"
