# Plugins and local core development

Use this reference for plugin installation, custom module plugins, native build compatibility, and plugin-to-template composition. The matching core manual is `docs/PLUGINS.md`. Host-side source changes use the core checkout's own developer instructions; ordinary application authoring does not require a core rebuild.

## Names and files

Plugins may be native Go `.so` or WebAssembly `.wasm` artifacts. Global sources live under `plugins/<name>/<version>/`; custom sources live under `modules/<module>/plugins/<name>/<version>/`. Each version has `manifest.json`. Compiled artifacts normally live at the project root under `bin/plugins/`.

| Source | Artifact base / configured name |
| --- | --- |
| Global plugin | `<Binary>@<version>` |
| Custom module plugin | `<Binary>__<module>@<version>` |

Enable the exact artifact name without its extension:

```yaml
hyperbricks:
  plugins:
    enabled:
      - ExamplePlugin@1.0.0
      - CustomWidget__demo@1.0.0
  directories:
    plugins: ./bin/plugins
```

The same name selects the renderer in a component:

```yaml
widget:
  - type: plugin
  - plugin: CustomWidget__demo@1.0.0
  - data:
      title: Project overview
```

These are naming examples, not guaranteed public registry entries. Keep the manifest's source name separate from the compiled binary name. Building a plugin does not automatically add it to `package.hyperbricks.yaml`.

## CLI workflow

Inspect available plugins and flags:

```sh
hyperbricks plugin list
hyperbricks plugin build --help
```

For a real selected plugin, the command shapes are:

```sh
hyperbricks plugin install example@1.0.0
hyperbricks plugin build example@1.0.0
hyperbricks plugin build widget@1.0.0 --module demo
```

`install` gets/builds a global plugin; `build` compiles existing source. `update <name>` selects a compatible global update. `remove <name>@<version>` removes an artifact; use `--module demo` for custom plugins. Only perform those mutations when they belong to the user's task.

A native plugin must match the running HyperBricks build and compatible Go build dependencies/toolchain. Rebuild after runtime API changes or compatibility errors. WASM uses a different ABI; it is not unrestricted native Go running in another file format. Consult the manual for its host capability and resource limits. If both `.so` and `.wasm` exist for the same configured name, startup rejects the ambiguity.

## Manifest

A native plugin manifest has this shape (adapt names/version/compatibility):

```json
{
  "plugin": "github.com/example/site/widget",
  "source": "widget_plugin.go",
  "runtime": "native",
  "version": "1.0.0",
  "binary": "CustomWidget",
  "compatible_hyperbricks": [">=1.2.2-beta"],
  "description": "Project-specific widget"
}
```

Required fields are `plugin`, `source`, `version`, `compatible_hyperbricks`, and `description`. `binary` is optional; without it the source stem is converted to CamelCase. `runtime` defaults to native and can select WASM. Set compatibility from tested runtime versions; the sample range is not a compatibility claim.

## Module-local source versus local runtime

`HYPERBRICKS_LOCAL_PATH` is a development override for targeting local HyperBricks source. Leave it unset when building plugins for an installed published release. Compiling plugin source alone does not require the override; it selects the HyperBricks dependency, not the plugin directory.

A CLI installed from a checkout with `go install ./cmd/hyperbricks` is still a local-source build. Build its plugins against that same checkout:

```sh
export HYPERBRICKS_LOCAL_PATH=/absolute/path/to/hyperbricks
hyperbricks plugin build widget@1.0.0 --module demo
```

The environment override works for `build` and global plugin `install`. The current CLI exposes `--hyperbricks-path` on `plugin install`, but not on `plugin build`; check the chosen command's help before using it. Run a runtime binary built from that checkout for the verification. A development binary's embedded version may still show an older release; record the actual source revision. Without a local override, release builds need a real available tag/revision. An `unknown revision` failure is not a reason to invent and publish a temporary tag.

## Keep YAML routes and template HTML visible

Several explicit fragment/action routes can invoke the same plugin with an action field in `data`. The plugin owns the workflow and validates the action; YAML owns which route calls it. This avoids hiding application routing inside an unrelated template.

For plugins returning template configuration, pass the template through YAML so it is preloaded:

```yaml
panel:
  - type: plugin
  - plugin: CustomWidget__demo@1.0.0
  - data:
      template:
        file: project-panel.html
```

The plugin receives the template name, then can return a runtime configuration map with `"@type": "<TEMPLATE>"`, `template`, and `values`. A `TREE` wrapper is only needed for multiple composed outputs. Runtime `@type` is appropriate in Go code returning runtime maps; YAML authors still use `type: template`.

Use the integrated dashboard's optional plugin lesson for a complete, runnable source/manifest/template example, with its documented build prerequisites. A plugin should enforce permissions around its own data operations, even when its route already has a guard.

## Diagnose by the contract that failed

- Missing plugin: selected module's enabled list, exact name including `__<module>@<version>`, actual artifact, and configured plugin directory.
- Wrong directory: the module flag does not change the working directory; `./bin/plugins` remains project-relative.
- Compatibility error: runtime binary source revision/toolchain and the plugin's build inputs; rebuild with the appropriate local override when needed.
- Empty rendered output: plugin's return contract, preloaded template name, values, and component errors. Keep debug output out of the rendered template.
