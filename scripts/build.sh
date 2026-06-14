#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# Navigate to the project root directory (assuming `scripts` is a subdirectory of the root)
cd "$(dirname "$0")/.."

GO_REQUIRED_VERSION="${GO_REQUIRED_VERSION:-go1.26.1}"
GO_BIN="${GO_BIN:-go}"
GO_VERSION="$("$GO_BIN" env GOVERSION 2>/dev/null || true)"
if [ "$GO_VERSION" != "$GO_REQUIRED_VERSION" ]; then
  echo "Go $GO_REQUIRED_VERSION is required, got ${GO_VERSION:-unknown}. Set GO_BIN to the matching go binary." >&2
  exit 1
fi

# Enable SQLite3 interface
set CGO_ENABLED=1
"$GO_BIN" env -w CGO_ENABLED=1

echo "Current directory: $(pwd)"
echo "Building plugins"

# debug Build plugins
# https://youtrack.jetbrains.com/issue/GO-6288/Debugger-support-with-plugins#focus=Comments-27-3517329.0-0
"$GO_BIN" build -gcflags "all=-N -l" -buildmode=plugin -o ./bin/plugins/debug/LoremIpsumPlugin.so ./plugins/loremipsum/lorem_ipsum_plugin.go
"$GO_BIN" build -gcflags "all=-N -l" -buildmode=plugin -o ./bin/plugins/debug/MarkDownPlugin.so ./plugins/markdown/markdown_plugin.go

#  Build plugins
# https://youtrack.jetbrains.com/issue/GO-6288/Debugger-support-with-plugins#focus=Comments-27-3517329.0-0
"$GO_BIN" build  -buildmode=plugin -o ./bin/plugins/LoremIpsumPlugin.so ./plugins/loremipsum/lorem_ipsum_plugin.go
"$GO_BIN" build  -buildmode=plugin -o ./bin/plugins/MarkDownPlugin.so ./plugins/markdown/markdown_plugin.go


# Build hyperbricks cms via install
"$GO_BIN" install -ldflags="-s -w" ./cmd/hyperbricks
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "$GO_BIN" build -o ./bin/linux/hyperbricks ./cmd/hyperbricks

# Build hyperbricks cms for linux
echo "Build complete!\n\n"
