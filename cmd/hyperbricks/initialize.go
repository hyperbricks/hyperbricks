package main

import (
	"context"
	"fmt"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

func init() {
	commands.RegisterSubcommands()

	// Execute the root command
	if err := commands.Execute(); err != nil {
		fmt.Println(err)
	}

	if !commands.StartMode {
		return
	}

	shared.Init_configuration()
	shared.Module = fmt.Sprintf("modules/%s/package.hyperbricks", commands.StartModule)
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
	logging.GetLogger().Info(orangeTrueColor, logo, reset)

	switch hbConfig.Mode {
	case shared.DEBUG_MODE:
		debug_mode_init()
	case shared.LIVE_MODE:
		live_mode_init()
	case shared.DEVELOPMENT_MODE:
		development_mode_init()
	}

}

// Package-level channel declaration
var (
	ctx    context.Context
	cancel context.CancelFunc
)

func initialisation(passedCtx context.Context, passedCancel context.CancelFunc) {
	ctx = passedCtx
	cancel = passedCancel

	setWorkingDirectory()
	applyHyperBricksConfigurations()

	//First initialize all render components, because they have to be registered befor parsing.
	initializeComponents()
	PreProcessAndPopulateHyperbricksConfigurations()

	// now configure the registered renderers with acquired configurations
	configureRenderers()

	//InitStaticFileServer()
	initStaticFileServer()

	// Now everything is ready start the server
	StartServer(ctx)

}
