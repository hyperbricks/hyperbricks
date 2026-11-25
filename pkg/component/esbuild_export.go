package component

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// exportInputPlugin makes builds reading static output consume this export's
// bytes. esbuild still resolves imports using their authored location, including
// imports that leave static for a source directory or an installed package.
func (s esbuildSpec) exportInputPlugin() api.Plugin {
	return api.Plugin{Name: "hyperbricks-export-inputs", Setup: func(build api.PluginBuild) {
		resolvedSource, sourceErr := esbuildExistingPath(s.SourceStaticDir)
		resolvedOutput, outputErr := esbuildExistingPath(s.StaticDir)
		resolving := &struct{ active bool }{true}
		mapSource := func(path string) (string, bool) {
			if mapped, ok := esbuildMapRoot(path, s.SourceStaticDir, s.StaticDir); ok {
				return mapped, true
			}
			return esbuildMapRoot(path, resolvedSource, s.StaticDir)
		}
		build.OnResolve(api.OnResolveOptions{Filter: ".*"}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			if args.PluginData == resolving {
				return api.OnResolveResult{}, nil
			}
			if sourceErr != nil {
				return api.OnResolveResult{}, sourceErr
			}
			if outputErr != nil {
				return api.OnResolveResult{}, outputErr
			}
			options := api.ResolveOptions{Importer: args.Importer, Namespace: args.Namespace, ResolveDir: args.ResolveDir, Kind: args.Kind, With: args.With, PluginData: resolving}
			original := build.Resolve(args.Path, options)
			// Respect external patterns at the original authored location before
			// considering an output-root substitution.
			if original.External {
				return esbuildResolvedImport(original), nil
			}
			candidate := args.Path
			if !filepath.IsAbs(candidate) && (strings.HasPrefix(candidate, ".") || args.Kind == api.ResolveEntryPoint) {
				dir := args.ResolveDir
				if dir == "" {
					dir = s.WorkingDir
				}
				candidate = filepath.Join(dir, candidate)
			}
			if mapped, ok := mapSource(candidate); ok {
				return esbuildResolvedImport(build.Resolve(mapped, options)), nil
			}
			if original.Namespace == "file" {
				if mapped, ok := mapSource(original.Path); ok {
					return esbuildResolvedImport(build.Resolve(mapped, options)), nil
				}
			}
			return api.OnResolveResult{}, nil
		})
		build.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "file"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
			authored, ok := esbuildMapRoot(args.Path, s.StaticDir, s.SourceStaticDir)
			if !ok {
				authored, ok = esbuildMapRoot(args.Path, resolvedOutput, s.SourceStaticDir)
			}
			if !ok {
				return api.OnLoadResult{}, nil
			}
			data, err := os.ReadFile(args.Path)
			if err != nil {
				return api.OnLoadResult{}, err
			}
			contents := string(data)
			return api.OnLoadResult{Contents: &contents, ResolveDir: filepath.Dir(authored), Loader: api.LoaderDefault}, nil
		})
	}}
}

func esbuildMapRoot(path, from, to string) (string, bool) {
	if !filepath.IsAbs(path) || from == "" {
		return "", false
	}
	rel, err := filepath.Rel(from, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.Join(to, rel), true
}

func (s esbuildSpec) entryInput() (string, error) {
	if s.SourceStaticDir == "" {
		return s.Entry, nil
	}
	if mapped, ok := esbuildMapRoot(s.Entry, s.SourceStaticDir, s.StaticDir); ok {
		return mapped, nil
	}
	root, err := esbuildExistingPath(s.SourceStaticDir)
	if err != nil {
		return "", err
	}
	if mapped, ok := esbuildMapRoot(s.Entry, root, s.StaticDir); ok {
		return mapped, nil
	}
	return s.Entry, nil
}

func esbuildResolvedImport(result api.ResolveResult) api.OnResolveResult {
	sideEffects := api.SideEffectsTrue
	if !result.SideEffects {
		sideEffects = api.SideEffectsFalse
	}
	return api.OnResolveResult{Path: result.Path, Namespace: result.Namespace, External: result.External, Suffix: result.Suffix, Errors: result.Errors, Warnings: result.Warnings, SideEffects: sideEffects}
}
