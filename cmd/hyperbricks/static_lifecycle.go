package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func runStaticOperation(config *shared.Config) error {
	plan, err := planStaticExport(config)
	if errors.Is(err, errStaticDeclined) {
		return nil
	}
	if err != nil {
		return err
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
	return runStaticLifecycle(ctx, config, plan, commands.StaticWithProcesses, basic_initialisation, func() (staticExportResult, error) {
		configMutex.RLock()
		routes := configs
		configMutex.RUnlock()
		return executeStaticExportContext(ctx, routes, plan)
	}, func() error {
		if commands.ServeStatic {
			return serveStaticContext(ctx)
		}
		return nil
	})
}

// The CLI owns the operation. Callbacks make stage order and failure behavior
// testable without substituting the process runner or publishing anything.
func runStaticLifecycle(ctx context.Context, config *shared.Config, plan staticExportPlan, enabled bool, initialize func() error, export func() (staticExportResult, error), serve func() error) (result error) {
	phase := "before_static"
	artifact := staticExportResult{RenderDir: plan.RenderDir}
	var processes *developmentProcesses
	defer func() {
		closeRuntimeAssets()
		if processes != nil {
			if err := processes.Stop(); err != nil {
				if result == nil || onlyOperationCancellation(result) {
					phase = "cleanup"
				}
				result = errors.Join(result, phaseError("cleanup", err))
			}
		}
		if result == nil || onlyOperationCancellation(result) {
			if cause := context.Cause(ctx); cause != nil {
				result = cause
			}
		}
		result = finishOperation(config, enabled, "static", phase, artifact, result)
	}()
	if enabled {
		var err error
		processes, err = newDevelopmentProcesses(commands.GetModuleRoot(), 0)
		if err != nil {
			return err
		}
		processes.hookContext = map[string]string{"HB_OPERATION": "static", "HB_RENDER_DIR": plan.RenderDir}
		if err = processes.RunTasks(ctx, "before_static", config.Hooks.BeforeStatic); err != nil {
			return err
		}
	} else if len(config.Hooks.BeforeStatic)+len(config.Hooks.AfterStatic)+len(config.Hooks.Finish) > 0 {
		logging.GetLogger().Info("Static hooks are disabled; use static --with-processes to enable them")
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	phase = "initialize"
	if err := initialize(); err != nil {
		return err
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	phase = "render"
	var err error
	artifact, err = export()
	if err != nil {
		return err
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	phase = "after_static"
	if processes != nil {
		processes.hookContext["HB_EXPORT_ZIP"] = artifact.ZipPath
		if err = processes.RunTasks(ctx, "after_static", config.Hooks.AfterStatic); err != nil {
			return err
		}
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	phase = "serve"
	return serve()
}

func finishOperation(config *shared.Config, enabled bool, operation, phase string, artifact staticExportResult, original error) error {
	if !enabled || len(config.Hooks.Finish) == 0 {
		return original
	}
	outcome, failed, code := "success", "", 0
	if original != nil {
		outcome, failed, code = "failure", phase, 1
		if primary := primaryOperationFailure(original); primary != nil {
			var stage *operationPhaseError
			if errors.As(primary, &stage) {
				failed = stage.Phase
			}
		}
		if errors.Is(original, errRuntimeInterrupt) {
			code = 130
		}
		if onlyOperationCancellation(original) {
			outcome, failed = "cancelled", ""
			if code != 130 {
				code = 0
			}
		}
	}
	processes, err := newDevelopmentProcesses(commands.GetModuleRoot(), config.Server.Port)
	if err != nil {
		return errors.Join(original, err)
	}
	processes.hookContext = map[string]string{
		"HB_OPERATION": operation, "HB_OUTCOME": outcome, "HB_FAILED_PHASE": failed, "HB_EXIT_CODE": strconv.Itoa(code), "HB_RENDER_DIR": artifact.RenderDir, "HB_EXPORT_ZIP": artifact.ZipPath,
	}
	var limit time.Duration
	for _, task := range config.Hooks.Finish {
		timeout := task.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		limit += timeout + 10*time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	taskErr := processes.RunTasks(ctx, "finish", config.Hooks.Finish)
	stopErr := processes.Stop()
	if taskErr != nil || stopErr != nil {
		return errors.Join(original, fmt.Errorf("finish: %w", errors.Join(taskErr, stopErr)))
	}
	return original
}

type operationPhaseError struct {
	Phase string
	Err   error
}

func (e *operationPhaseError) Error() string { return e.Phase + ": " + e.Err.Error() }
func (e *operationPhaseError) Unwrap() error { return e.Err }
func phaseError(phase string, err error) error {
	if err == nil {
		return nil
	}
	return &operationPhaseError{phase, err}
}

func onlyOperationCancellation(err error) bool {
	if err == nil {
		return false
	}
	if err == context.Canceled || err == errRuntimeInterrupt {
		return true
	}
	if multiple, ok := err.(interface{ Unwrap() []error }); ok {
		children := multiple.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !onlyOperationCancellation(child) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return onlyOperationCancellation(wrapped.Unwrap())
	}
	return false
}

// Joined cleanup errors must not hide an earlier operation failure. A pure
// cancellation is skipped so a real cleanup failure remains visible.
func primaryOperationFailure(err error) error {
	if err == nil || onlyOperationCancellation(err) {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if primary := primaryOperationFailure(child); primary != nil {
				return primary
			}
		}
		return nil
	}
	return err
}
