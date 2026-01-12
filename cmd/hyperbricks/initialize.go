package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"golang.org/x/time/rate"
)

func init() {
	if isTestRun() || flag.Lookup("test.v") != nil {
		return
	}

	runtime.GOMAXPROCS(4)

	commands.RegisterSubcommands()
	commands.PluginCommand()

	// Execute the root command
	if err := commands.Execute(); err != nil {
		fmt.Println(err)
	}

	// exit if Version or Plugin command
	if commands.Exit {
		return
	}

	shared.Init_configuration()

	shared.Module = commands.GetModuleConfigPath()
	hbConfig := getHyperBricksConfiguration()

	orangeTrueColor := "\033[38;2;255;165;0m"
	reset := "\033[0m"
	logo := `
 _   _                       ____       _      _        
| | | |_   _ _ __   ___ _ __| __ ) _ __(_) ___| | _____ 
| |_| | | | | '_ \ / _ \ '__|  _ \| '__| |/ __| |/ / __|
|  _  | |_| | |_) |  __/ |  | |_) | |  | | (__|   <\__ \
|_| |_|\__, | .__/ \___|_|  |____/|_|  |_|\___|_|\_\___/
       |___/|_|                                        

`
	logging.GetLogger().Info(orangeTrueColor, fmt.Sprintf(`%s%s`, logo, assets.VersionMD), reset)

	if commands.RenderStatic {
		basic_initialisation()

		// serve
		if commands.ServeStatic {
			if err := serveStatic(); err != nil {
				log.Fatalf("Static server error: %v", err)
			}
		}
	}

	if !commands.StartMode {
		return
	}

	switch hbConfig.Mode {
	case shared.DEBUG_MODE:
		debug_mode_init()
	case shared.LIVE_MODE:
		live_mode_init()
	case shared.DEVELOPMENT_MODE:
		development_mode_init()
	}

}

func isTestRun() bool {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}
	return false
}

// Package-level channel declaration
var (
	ctx    context.Context
	cancel context.CancelFunc
)

func initialisation(passedCtx context.Context, passedCancel context.CancelFunc) {
	ctx = passedCtx
	cancel = passedCancel

	basic_initialisation()

	hbConfig := getHyperBricksConfiguration()
	limiter := rate.NewLimiter(rate.Limit(hbConfig.RateLimit.RequestsPerSecond), hbConfig.RateLimit.Burst)

	// Initialize Static File Server with Rate Limiting
	initStaticFileServer(limiter)

	// Now everything is ready, start the server
	StartServer(ctx)

}

// minimal initialisation (also for static rendering)
func basic_initialisation() {
	setWorkingDirectory()
	applyHyperBricksConfigurations()

	// First initialize all render components, because they have to be registered before parsing.
	initializeComponents()

	// Now configure and populate the registered renderers with acquired configurations
	PreProcessAndPopulateHyperbricksConfigurations()
}
