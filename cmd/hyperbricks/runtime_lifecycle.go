package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

var errRuntimeInterrupt = errors.New("interrupted")

func validateProcessLaunch(enabled bool, mode string, managed bool) error {
	if !enabled {
		return nil
	}
	if managed {
		return errors.New("--with-processes is unavailable for deployment-managed runtimes")
	}
	if mode != shared.DEVELOPMENT_MODE && mode != shared.DEBUG_MODE {
		return errors.New("--with-processes requires development or debug mode")
	}
	return nil
}

func runRuntimeMode(config *shared.Config) {
	managed := commands.DeployBuildID != "" || os.Getenv("HB_DEPLOY_BUILD_ID") != "" || os.Getenv("HB_DEPLOY_MODULE") != ""
	if err := validateProcessLaunch(commands.StartWithProcesses, config.Mode, managed); err != nil {
		commands.ReportError(err)
		return
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		select {
		case sig := <-signals:
			if sig == os.Interrupt {
				cancel(errRuntimeInterrupt)
			} else {
				cancel(context.Canceled)
			}
		case <-ctx.Done():
		}
	}()
	reloads := newRuntimeReload(PreProcessAndPopulateHyperbricksConfigurations)
	go keyboardActions(ctx, cancel, reloads)
	err := runRuntimeSession(ctx, config, commands.StartWithProcesses, reloads)
	switch err {
	case nil, context.Canceled:
	case errRuntimeInterrupt:
		commands.ExitCode = 130
	default:
		commands.ReportError(err)
	}
}

// The CLI session owns children, reloads and HTTP. Configuration and renderers
// have no authority to launch processes, including during static export.
func runRuntimeSession(parent context.Context, config *shared.Config, enabled bool, reloads *runtimeReload) (result error) {
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	logger := logging.GetLogger().Named("server")
	var processes *developmentProcesses
	var running *runtimeHTTPServer
	var stopCacheControl func(context.Context) error
	var stopCacheCleanup context.CancelFunc
	var cacheCleanupDone chan struct{}
	cacheConfigured := false
	watchCtx, stopWatch := context.WithCancel(context.Background())
	var watchDone <-chan error
	defer func() {
		logger.Info("Stopping")
		stopWatch()
		reloadCtx, stopReload := context.WithTimeout(context.Background(), 5*time.Second)
		cleanupErr := reloads.Stop(reloadCtx)
		stopReload()
		if watchDone != nil {
			select {
			case err := <-watchDone:
				cleanupErr = errors.Join(cleanupErr, err)
			case <-time.After(5 * time.Second):
				cleanupErr = errors.Join(cleanupErr, errors.New("file watcher did not stop"))
			}
		}
		if running != nil {
			shutdownCtx, stopHTTP := context.WithTimeout(context.Background(), 5*time.Second)
			cleanupErr = errors.Join(cleanupErr, running.shutdown(shutdownCtx))
			stopHTTP()
		}
		if stopCacheControl != nil {
			controlCtx, stopControl := context.WithTimeout(context.Background(), 5*time.Second)
			cleanupErr = errors.Join(cleanupErr, stopCacheControl(controlCtx))
			stopControl()
		}
		if stopCacheCleanup != nil {
			stopCacheCleanup()
			<-cacheCleanupDone
		}
		if cacheConfigured {
			cleanupErr = errors.Join(cleanupErr, closeResponseCache())
		}
		if processes != nil {
			cleanupErr = errors.Join(cleanupErr, processes.Stop())
		}
		if result == nil || result == context.Canceled {
			result = context.Cause(ctx)
		}
		if cleanupErr != nil {
			result = errors.Join(result, fmt.Errorf("runtime cleanup: %w", cleanupErr))
		}
		logger.Info("Stopped")
	}()

	if enabled && config.HasDevelopmentProcesses() {
		var err error
		processes, err = newDevelopmentProcesses(commands.GetModuleRoot(), config.Server.Port)
		if err != nil {
			return err
		}
		go func() {
			select {
			case err := <-processes.Failures():
				if err != nil {
					cancel(err)
				}
			case <-ctx.Done():
			}
		}()
		if err := processes.RunTasks(ctx, "before_start", config.Development.Hooks.BeforeStart); err != nil {
			return err
		}
		if err := processes.StartServices(ctx, config.Development.Services); err != nil {
			return err
		}
	} else if config.HasDevelopmentProcesses() {
		logger.Info("Development hooks and services are disabled; use start --with-processes to enable them")
	}

	// Preparation may execute a compiler/plugin. Cancellation must still let the
	// CLI clean up its children if an in-process dependency does not cooperate.
	prepared := make(chan error, 1)
	go func() { prepared <- basic_initialisation() }()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case err := <-prepared:
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if config.Mode != shared.LIVE_MODE {
		statusServer()
	}
	if config.Mode == shared.LIVE_MODE {
		cacheRoot, cacheErr := responseCacheDirectory(commands.GetModuleRoot(), config)
		if cacheErr != nil {
			logger.Warnw("Disk response cache unavailable; disk routes will render fresh", "error", cacheErr)
			_ = closeResponseCache()
		} else if err := configureResponseCache(commands.GetModuleRoot(), cacheRoot, config.Live.DiskCache.MaxBytes, config.Live.DiskCache.MaxEntries); err != nil {
			logger.Warnw("Disk response cache unavailable; disk routes will render fresh", "error", err)
		}
		cacheConfigured = true
		cleanupCtx, stopCleanup := context.WithCancel(context.Background())
		stopCacheCleanup = stopCleanup
		cacheCleanupDone = make(chan struct{})
		go func() {
			defer close(cacheCleanupDone)
			runResponseCacheCleanup(cleanupCtx, config.Live.DiskCache.CleanupInterval)
		}()
		var err error
		stopCacheControl, err = startCacheControl(commands.GetModuleRoot())
		if err != nil {
			logger.Warnw("Local cache purge control unavailable", "error", err)
		}
	}
	initStaticFileServer(newRequestRateLimiter(config.RateLimit))
	var err error
	running, err = startRuntimeHTTPServer()
	if err != nil {
		return err
	}
	go func() {
		select {
		case <-running.done:
			if running.err != nil && !errors.Is(running.err, http.ErrServerClosed) {
				cancel(fmt.Errorf("HTTP server: %w", running.err))
			} else {
				cancel(context.Canceled)
			}
		case <-ctx.Done():
		}
	}()
	if processes != nil {
		if err := processes.RunTasks(ctx, "after_start", config.Development.Hooks.AfterStart); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if config.Mode != shared.LIVE_MODE && config.Development.Watch {
		reloads.Enable()
	}
	if config.Mode == shared.DEVELOPMENT_MODE && config.Development.Watch {
		watchDone, err = startRuntimeWatcher(watchCtx, resolveDevelopmentWatchDirectories(config), reloads.Request, config.Directories["cache"])
		if err != nil {
			return err
		}
	}
	logger.Info("Started")
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case err := <-watchDone:
		watchDone = nil // consumed; cleanup must not wait for it a second time
		if err != nil {
			return fmt.Errorf("file watcher: %w", err)
		}
		return errors.New("file watcher stopped unexpectedly")
	}
}
