package main

import (
	"context"
	_ "embed"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// A before_start task writes this file before invoking the existing plugin CLI.
// Its value is compiled into the artifact, proving which build was loaded.
//go:embed build-id.txt
var buildID string

type greetingPlugin struct{}

func (*greetingPlugin) Render(_ interface{}, _ context.Context) (any, []error) {
	return "Hooks plugin built before load: " + strings.TrimSpace(buildID), nil
}

func Plugin() (shared.PluginRenderer, error) {
	return &greetingPlugin{}, nil
}
