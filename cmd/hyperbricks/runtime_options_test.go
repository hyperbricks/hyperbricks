package main

import (
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func resetCommandRuntimeOptionsForTest(t *testing.T) {
	t.Helper()

	previousRuntimeOptions := shared.GetRuntimeOptions()
	previousStartModule := commands.StartModule
	previousModuleRoot := commands.ModuleRoot
	previousPort := commands.Port
	previousProduction := commands.Production
	previousRuntimeGateway := commands.StartRuntimeGateway
	previousRuntimeDomain := commands.StartRuntimeDomain
	previousRuntimeHostSuffix := commands.StartRuntimeHostSuffix
	previousRuntimeResolver := commands.StartRuntimeResolver

	shared.SetRuntimeOptions(shared.RuntimeOptions{})
	commands.StartModule = ""
	commands.ModuleRoot = ""
	commands.Port = 8080
	commands.Production = false
	commands.StartRuntimeGateway = false
	commands.StartRuntimeDomain = ""
	commands.StartRuntimeHostSuffix = ""
	commands.StartRuntimeResolver = ""

	t.Cleanup(func() {
		shared.SetRuntimeOptions(previousRuntimeOptions)
		commands.StartModule = previousStartModule
		commands.ModuleRoot = previousModuleRoot
		commands.Port = previousPort
		commands.Production = previousProduction
		commands.StartRuntimeGateway = previousRuntimeGateway
		commands.StartRuntimeDomain = previousRuntimeDomain
		commands.StartRuntimeHostSuffix = previousRuntimeHostSuffix
		commands.StartRuntimeResolver = previousRuntimeResolver
	})
}

func TestApplyCommandRuntimeOptionsCopiesCommandState(t *testing.T) {
	resetCommandRuntimeOptionsForTest(t)

	commands.ModuleRoot = "deploy/demo/runtime/current"
	commands.Port = 9099
	commands.Production = true
	commands.StartRuntimeGateway = true
	commands.StartRuntimeDomain = "runtime.local"
	commands.StartRuntimeHostSuffix = "-runtime.local"
	commands.StartRuntimeResolver = "http://resolver.local/resolve"

	applyCommandRuntimeOptions()

	options := shared.GetRuntimeOptions()
	if options.ModuleRoot != "deploy/demo/runtime/current" {
		t.Fatalf("module root = %q, want deploy/demo/runtime/current", options.ModuleRoot)
	}
	if options.Port != 9099 || !options.PortOverride {
		t.Fatalf("port override = (%d, %v), want (9099, true)", options.Port, options.PortOverride)
	}
	if !options.Production {
		t.Fatal("production = false, want true")
	}
	if !options.RuntimeGatewayEnabled {
		t.Fatal("runtime gateway enabled = false, want true")
	}
	if options.RuntimeGatewayDomain != "runtime.local" {
		t.Fatalf("runtime gateway domain = %q, want runtime.local", options.RuntimeGatewayDomain)
	}
	if options.RuntimeGatewayHostSuffix != "-runtime.local" {
		t.Fatalf("runtime gateway host suffix = %q, want -runtime.local", options.RuntimeGatewayHostSuffix)
	}
	if options.RuntimeGatewayResolver != "http://resolver.local/resolve" {
		t.Fatalf("runtime gateway resolver = %q, want resolver URL", options.RuntimeGatewayResolver)
	}
}

func TestApplyCommandRuntimeOptionsDefaultPortDoesNotOverride(t *testing.T) {
	resetCommandRuntimeOptionsForTest(t)

	commands.StartModule = "demo"
	commands.Port = 8080

	applyCommandRuntimeOptions()

	options := shared.GetRuntimeOptions()
	if options.ModuleRoot != "modules/demo" {
		t.Fatalf("module root = %q, want modules/demo", options.ModuleRoot)
	}
	if options.Port != 8080 {
		t.Fatalf("port = %d, want default CLI port 8080", options.Port)
	}
	if options.PortOverride {
		t.Fatal("port override = true, want false for default CLI port")
	}
}
