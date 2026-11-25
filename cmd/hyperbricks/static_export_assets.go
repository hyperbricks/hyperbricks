package main

import (
	"context"
	"path/filepath"

	"github.com/hyperbricks/hyperbricks/pkg/component"
)

// The CLI owns one export operation. Its asset instances are created before
// hooks, then reused when the rest of the runtime is initialized after hooks.
// Source configuration retains its original directory values throughout.
type staticExportAssets struct {
	plan     staticExportPlan
	renderer *component.EsbuildRenderer
	images   *component.ImageProcessor
}

var activeStaticExport *staticExportAssets

func runtimeStaticDirectory() string {
	if activeStaticExport != nil {
		return filepath.Join(activeStaticExport.plan.RenderDir, "static")
	}
	return getHyperBricksConfiguration().Directories["static"]
}

func openStaticExportAssets(plan staticExportPlan) (*staticExportAssets, error) {
	output := filepath.Join(plan.RenderDir, "static")
	assets := &staticExportAssets{plan: plan, renderer: component.NewEsbuildRenderer(plan.StaticDir), images: component.NewImageProcessor()}
	if err := assets.renderer.SetOutputRoot(output); err != nil {
		return nil, err
	}
	if err := assets.images.SetOutputRoot(plan.StaticDir, output); err != nil {
		return nil, err
	}
	if err := assets.renderer.BeginAssetSession(); err != nil {
		return nil, err
	}
	if err := assets.images.BeginAssetSession(filepath.Join(output, "images")); err != nil {
		assets.renderer.CloseAssets()
		return nil, err
	}
	return assets, nil
}

func (assets *staticExportAssets) close() {
	assets.images.CloseAssets()
	assets.renderer.CloseAssets()
}

func (assets *staticExportAssets) reset(ctx context.Context) error {
	if err := assets.renderer.ResetAssetsForFreshExport(ctx); err != nil {
		return err
	}
	return assets.images.ResetAssetsForFreshExport(ctx, filepath.Join(assets.plan.RenderDir, "static", "images"))
}

func withStaticExportAssets(ctx context.Context, plan staticExportPlan, operation func(*staticExportAssets) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	unlock, err := component.LockAssetSession(plan.RenderDir, "export")
	if err != nil {
		return phaseError("asset_cleanup", err)
	}
	defer unlock()
	assets, err := openStaticExportAssets(plan)
	if err != nil {
		return phaseError("asset_cleanup", err)
	}
	defer assets.close()
	previous := activeStaticExport
	activeStaticExport = assets
	defer func() { activeStaticExport = previous }()
	return operation(assets)
}

func (assets *staticExportAssets) prepareOutput(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Ownership from a previous, now discarded export must not adopt or prune
	// source files copied into this fresh operation.
	if err := assets.reset(ctx); err != nil {
		return phaseError("asset_cleanup", err)
	}
	if err := component.ResetAndCopyStaticAssets(ctx, assets.plan.StaticDir, assets.plan.RenderDir); err != nil {
		return phaseError("copy_assets", err)
	}
	return nil
}
