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

# debug Build plugins
# https://youtrack.jetbrains.com/issue/GO-6288/Debugger-support-with-plugins#focus=Comments-27-3517329.0-0
go build -gcflags "all=-N -l" -buildmode=plugin -o ./bin/plugins/debug/LoremIpsumPlugin.so ./plugins/loremipsum/lorem_ipsum_plugin.go
go build -gcflags "all=-N -l" -buildmode=plugin -o ./bin/plugins/debug/MarkDownPlugin.so ./plugins/markdown/markdown_plugin.go

#  Build plugins
# https://youtrack.jetbrains.com/issue/GO-6288/Debugger-support-with-plugins#focus=Comments-27-3517329.0-0
go build  -buildmode=plugin -o ./bin/plugins/LoremIpsumPlugin.so ./plugins/loremipsum/lorem_ipsum_plugin.go
go build  -buildmode=plugin -o ./bin/plugins/MarkDownPlugin.so ./plugins/markdown/markdown_plugin.go


# Build hyperbricks cms
#go build -v -o ./bin/hyperbricks ./cmd/hyperbricks

echo "Build complete!\n\n"
