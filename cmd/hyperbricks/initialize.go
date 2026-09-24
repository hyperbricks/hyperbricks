package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
)

func run() {
	defer func() {
		if err := logging.Close(); err != nil {
			commands.ReportError(fmt.Errorf("close log output: %w", err))
		}
	}()
	commands.RegisterSubcommands()

	// Execute the root command
	if err := commands.Execute(); err != nil {
		commands.ReportError(err)
		return
	}

	// exit if Version or Plugin command
	if commands.Exit || (!commands.StartMode && !commands.RenderStatic) {
		return
	}

	shared.Init_configuration()
	applyCommandRuntimeOptions()

	if commands.DeployServiceMode == commands.DeployServiceRemote {
		if err := startDeployAPIServer(commands.GetDeployConfigPath()); err != nil {
			commands.ReportError(fmt.Errorf("start remote deployment service: %w", err))
		}
		return
	}
	if commands.DeployServiceMode == commands.DeployServiceLocal {
		if err := startDeployLocalServer(commands.GetDeployConfigPath()); err != nil {
			commands.ReportError(fmt.Errorf("start local deploy server: %w", err))
		}
		return
	}

	shared.Module = commands.GetModuleConfigPath()
	hbConfig := getHyperBricksConfiguration()
	if err := configureRuntimeLogging(hbConfig); err != nil {
		commands.ReportError(err)
		return
	}
	if err := hbConfig.ValidateDevelopmentDashboard(); err != nil {
		commands.ReportError(err)
		return
	}
	if err := hbConfig.ValidateFrontendEditing(); err != nil {
		commands.ReportError(err)
		return
	}
	if err := hbConfig.ValidateRuntimeSettings(); err != nil {
		commands.ReportError(err)
		return
	}
	if err := configureGoMaxProcs(hbConfig.Server.GoMaxProcs); err != nil {
		commands.ReportError(err)
		return
	}
	logRuntimeSummary(hbConfig)

	if commands.RenderStatic {
		basic_initialisation()

		// serve
		if commands.ServeStatic {
			if err := serveStatic(); err != nil {
				commands.ReportError(fmt.Errorf("serve static output: %w", err))
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

func applyCommandRuntimeOptions() {
	shared.SetRuntimeOptions(shared.RuntimeOptions{
		ModuleRoot: commands.GetModuleRoot(),

		Port:         int(commands.Port),
		PortOverride: commands.Port != 8080,

		Production: commands.Production,

		RuntimeGatewayEnabled:    commands.StartRuntimeGateway,
		RuntimeGatewayDomain:     commands.StartRuntimeDomain,
		RuntimeGatewayHostSuffix: commands.StartRuntimeHostSuffix,
		RuntimeGatewayResolver:   commands.StartRuntimeResolver,
	})
}

func configureRuntimeLogging(config *shared.Config) error {
	level, format := config.Logger.Level, config.Logger.Format
	if level == "" {
		level = "info"
		if config.Mode == shared.DEBUG_MODE {
			level = "debug"
		}
	}
	if format == "" {
		format = "console"
	}
	level = commands.EffectiveLogLevel(level)
	if commands.LogFormat != "" {
		format = commands.LogFormat
	}
	if err := logging.Configure(level, format); err != nil {
		return err
	}
	path := runtimeLogFile(config)
	if path != "" {
		if err := logging.AddFileOutput(path); err != nil {
			return fmt.Errorf("open log file %q: %w", path, err)
		}
	}
	if format == "console" && !commands.NonInteractive && os.Getenv("HB_NO_KEYBOARD") == "" && logging.GetLogger().Desugar().Core().Enabled(zapcore.InfoLevel) {
		logging.WriteHeading(commands.RootCmd.ErrOrStderr(), string(assets.VersionMD))
	}
	return nil
}

func initialisation(ctx context.Context) {
	basic_initialisation()

	hbConfig := getHyperBricksConfiguration()
	limiter := newRequestRateLimiter(hbConfig.RateLimit)

	// Initialize Static File Server with Rate Limiting
	initStaticFileServer(limiter)

	// Now everything is ready, start the server
	StartServer(ctx)

}

func newRequestRateLimiter(config shared.RateLimitConfig) *rate.Limiter {
	if !config.Enabled {
		return nil
	}
	return rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.Burst)
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
