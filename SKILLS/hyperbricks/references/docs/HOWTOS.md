<!-- Generated from docs/HOWTOS.md. Do not edit directly. -->

# How-to guides

## Contents

- [Where to find complete module examples](#where-to-find-complete-module-examples)
- [1. Start from a module](#1-start-from-a-module)
- [2. Reuse structure with imports and inheritance](#2-reuse-structure-with-imports-and-inheritance)
- [3. Define HTML structure in templates](#3-define-html-structure-in-templates)
- [4. Assign a URL to a composite component](#4-assign-a-url-to-a-composite-component)
- [5. Build assets through the native component](#5-build-assets-through-the-native-component)
- [6. Create a native Go plugin](#6-create-a-native-go-plugin)
- [7. Run JavaScript on the server with goja_render](#7-run-javascript-on-the-server-with-goja_render)
- [8. Display an API response as HTML](#8-display-an-api-response-as-html)
- [9. Use one plugin for different actions](#9-use-one-plugin-for-different-actions)
- [10. Protect a route with an access check](#10-protect-a-route-with-an-access-check)
- [11. Export a static site](#11-export-a-static-site)

## HTMX How-to

- [1. Enhance ordinary links with HTMX](#1-enhance-ordinary-links-with-htmx)
- [2. Describe navigation in configuration](#2-describe-navigation-in-configuration)

This section is for developers who know web development and are new to HyperBricks. See [YAML usage](YAML_USAGE.md) for source syntax and [Component reference](REFERENCE.md) for fields.

If an example fails, start with [Troubleshooting](TROUBLESHOOTING.md). For older configurations, see the [Migration Guide](MIGRATION.md).

## Where to find complete module examples

A **module** is a folder that groups the files for a HyperBricks application or example. It contains a `package.hyperbricks.yaml` file with the module’s settings, YAML component definitions, HTML templates, and resources such as CSS and JavaScript. A module can define several document and fragment routes.

Modules normally live inside the project’s `modules/` folder. For example, `modules/demo/` is the module named `demo`. The `-m demo` option tells HyperBricks which module to start, build, or export. One project can contain several modules, each with its own settings and application files.

A typical HyperBricks project layout looks like this:

```text
my-project/                   Project root
├── modules/                  Applications and examples
│   ├── demo/
│   ├── website/
│   └── another-module/
├── plugins/                  Shared plugin source code
│   └── myplugin/
│       └── 2.0.0/            Plugin version
└── bin/
    └── plugins/              Compiled plugins used by the runtime
```

Run HyperBricks commands from the project root and select the module with `-m`.


See the [module catalog](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/README.md) for an overview of the available modules, including learning applications, examples, integrations, and test fixtures. The patterns module has Go integration tests and an optional plugin smoke suite; the generated starter has CLI tests. Check this coverage when choosing an example: finding source code alone does not confirm that the complete application still runs.

The [YAML patterns module](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/hyperbricks-patterns-yaml/README.md) contains runnable examples of composition, navigation, fragments, access checks, API routes, and plugins. For a new application, `hyperbricks init` generates the maintained starting structure. You can extend that structure with your own database, services, and deployment setup.

## 1. Start from a module

Run the CLI from the project root, normally the directory containing `modules/`:

```sh
hyperbricks version
```

Choose one starting point:

- For a simple Hello World page, download the starter. This requires internet access:

  ```sh
  hyperbricks init-starter get hello-world -m my-project
  ```

- For the built-in starter with templates, fragments, and navigation, create the module with `init`:

  ```sh
  hyperbricks init -m my-project
  ```

Both commands create `modules/my-project/`. Run only one of them for that module, then start the server:

```sh
hyperbricks start -m my-project
```

Open `http://localhost:8080/` to see the output.

`init` provides a working scaffold. Its `package.hyperbricks.yaml` is ordinary YAML configuration; the files in `hyperbricks/` contain ordered component definitions. Keep templates in `templates/`, source assets in `resources/`, and public assets in `static/`. The `/static/` URL refers to the configured public directory; a `path` resolver returns a filesystem path, not a public URL.

Use `hyperbricks <command> --help` for the installed flags. A development checkout may contain features newer than its embedded version label; build and verify the example against that checkout before assuming another installation has the same components. See [CLI](HYPERBRICKS_CLI.md).

**Try it:** change the port, start the module, and open the new address. If a file is missing, check the selected module and configured directory first.

## 2. Reuse structure with imports and inheritance

Keep a small entry file that explicitly imports shared definitions. `*.hyperbricks.yaml` files directly inside the configured `hyperbricks/` directory load automatically; files in subdirectories need file-level `imports` in a loaded source file. `inherit` copies a named definition and applies local overrides.

```yaml
imports:
  - partials/site.hyperbricks.yaml

projects_page:
  - inherit: page_shell
  - route: projects
  - title: Projects
```

Here `page_shell` owns the shared document structure and `projects_page` supplies the route-specific values. Keep names semantic and references shallow enough to follow. Use ordinary maps for data and ordered sequences for component objects. Order matters for rendered children.

**Try it:** add a second page using the shell, then change the shared footer. Both pages should change. See [imports and inheritance](YAML_USAGE.md#imports).

## 3. Define HTML structure in templates

Keep the HTML structure in a template file and pass its content through `values` in YAML. Create `templates/card.html` in your module:

```html
<article>
  <h2>{{ .name }}</h2>
  <p>{{ .summary }}</p>
</article>
```

Use the `template` option with `file` to load it. The file path is relative to the module’s `templates/` folder:

```yaml
project_card:
  - type: template
  - template:
      file: card.html
  - values:
      name: Atlas
      summary: Shared project notes
```

`{{ .name }}` inserts the `name` value into the heading. `{{ .summary }}` inserts the `summary` value into the paragraph. Change these values in YAML to update the card’s text while keeping the same HTML structure.

For a small template, you can also put the HTML directly in YAML with `inline` instead of `template.file`:

```yaml
project_card:
  - type: template
  - inline: |
      <article>
        <h2>{{ .name }}</h2>
        <p>{{ .summary }}</p>
      </article>
  - values:
      name: Atlas
      summary: Shared project notes
```

A `template` does not have its own URL. Include `project_card` in a `hypermedia` or `fragment` component to render it at a route; see [Assign a URL to a composite component](#4-assign-a-url-to-a-composite-component).

**Try it:** change a card's data without editing its template. See [Template Syntax](YAML_USAGE.md#template-syntax).

## 4. Assign a URL to a composite component

A composite component combines content and rendering into an output. `hypermedia`, `fragment`, and `api_fragment_render` can serve that output at a URL. Set their `route` property to define the path: `route: projects` makes the component available at `/projects`.

- `hypermedia` creates a complete HTML document with a doctype, document structure, and configured head.
- `fragment` returns partial HTML without the document wrapper.
- `api_fragment_render` calls an API and renders its response as an HTML fragment.

Choose the component for the output the request needs. A document and a fragment can reuse the same template while serving different URLs. See [Routing](ROUTING.md).

This example defines an HTML document at `/projects`:

```yaml
projects_page:
  - type: hypermedia
  - route: projects
  - title: Projects
  - content:
      - type: html
      - value: <p>Welcome to our projects.</p>
```

The `route` assigns `/projects` to this component. `hypermedia` creates the HTML document, puts `Projects` in its document title, and renders the configured content inside the body.

## 5. Build assets through the native component

Keep source CSS and JavaScript under `resources/`. A native `esbuild` component builds them into `static/`; its result supplies the public URL. Use that result when fingerprints are enabled, since the generated filename can change.

Use ordinary CSS for the first project. Introduce tools such as Tailwind only when the application needs them. Put browser behavior in small resource files and use delegated events or appropriate HTMX lifecycle hooks when content is replaced. See [native esbuild](ESBUILD.md).

**Try it:** edit a color or browser label, let development reload rebuild the asset, and inspect the changed page. Refreshing generated output by hand should not be part of the workflow.

## 6. Create a native Go plugin

A native plugin runs compiled Go code inside the component tree. Use it when your application needs Go libraries, database access, or custom server logic. YAML supplies the plugin’s input and defines where its output is rendered.

To create a module-specific plugin, put its Go source and `manifest.json` in `modules/<module>/plugins/<name>/<version>/`. Build it with `hyperbricks plugin build <name>@<version> --module <module>`, enable the compiled artifact’s name under `hyperbricks.plugins.enabled` in `package.hyperbricks.yaml`, and reference that name in a `plugin` component.

The example in `plugins/myplugin/2.0.0/my_plugin.go` reads a message from YAML and returns it inside a `<div>`:

```go
package main

import (
	"context"
	"fmt"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// The plugin field definition
type Fields struct {
	Message string `mapstructure:"message"`
}

// Basic config for ComponentRenderers
type MyPluginConfig struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string `mapstructure:"plugin"`
	Fields           `mapstructure:"data"`
}

// MyPlugin implements the Renderer interface.
type MyPlugin struct{}

// Ensure MyPlugin implements shared.ComponentRenderer
var _ shared.PluginRenderer = (*MyPlugin)(nil)

// Render is the function that will be called by the renderer.
func (p *MyPlugin) Render(instance interface{}, ctx context.Context) (any, []error) {

	var errors []error

	var config MyPluginConfig
	err := shared.DecodeWithBasicHooks(instance, &config)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     config.HyperBricksPath,
			Key:      config.HyperBricksKey,
			Rejected: true,
			Err:      fmt.Sprintf("Failed to decode plugin instance: %v", err),
		})
		return "<!--Failed to render MyPlugin -->", errors
	}

	return fmt.Sprintf("<div class=\"my_plugin-content\">%s</div>\n", config.Fields.Message), errors
}

// var Plugin shared.PluginRenderer = &MyPlugin{}
// This function is exposed for the main application.
func Plugin() (shared.PluginRenderer, error) {
	return &MyPlugin{}, nil
}
```

`Plugin()` creates the plugin instance. HyperBricks calls `Render()` when it reaches the plugin component. `DecodeWithBasicHooks` reads the configuration, including `data.message`, into `MyPluginConfig`.

This is a global plugin, so its compiled name is `MyPlugin@2.0.0`. Build its source with `hyperbricks plugin build myplugin@2.0.0` and enable it in `package.hyperbricks.yaml`:

```yaml
hyperbricks:
  plugins:
    enabled:
      - MyPlugin@2.0.0
```

Reference the same name and supply a message in a page’s YAML:

```yaml
hello_page:
  - type: hypermedia
  - route: hello
  - title: Hello
  - greeting:
      - type: plugin
      - plugin: MyPlugin@2.0.0
      - data:
          message: Hello World!
```

The plugin renders `<div class="my_plugin-content">Hello World!</div>` as part of the document body.

See [Plugins](PLUGINS.md) for the implementation contract, manifest, configuration examples, and native build requirements.

## 7. Run JavaScript on the server with goja_render

The `goja_render` component runs JavaScript on the server and passes its result to an HTML template. Use it for small calculations, such as a total price or an availability message, using configured values and explicitly allowed query parameters. The browser receives the rendered HTML; no Node.js server or separate plugin build is needed.

For example, this script in `resources/scripts/availability.js` checks a configured stock value:

```javascript
function main(input) {
  const stock = Number(input.values.stock);
  return {
    message: stock > 0 ? "In stock" : "Out of stock"
  };
}
```

This component loads the script, supplies `stock`, and renders its returned message:

```yaml
availability:
  - type: goja_render
  - script:
      file:
        base: resources
        path: scripts/availability.js
  - values:
      stock: 12
  - inline: '<p>{{.Data.message}}</p>'
```

`input.values.stock` reads the configured value. The object returned by `main(input)` is available to the template as `.Data`. With `stock: 12`, the component produces:

```html
<p>In stock</p>
```

The component is in beta and intended for trusted project scripts. See [Goja Render](GOJA_RENDER.md) for query parameters, configuration options, and limitations.

## 8. Display an API response as HTML

API components call an API and pass its response to a template. Use `api_render` to include API data inside a document or fragment. Use `api_fragment_render` to give the API-backed HTML its own URL.

For example, this template in `templates/profile.html` displays a name when the API returns HTTP 200, or a message when it returns another status:

```html
{{if eq .Status 200}}
  <p>Hello, {{.Data.name}}.</p>
{{else}}
  <p>Could not load the profile. API status: {{.Status}}.</p>
{{end}}
```

This YAML connects the template to an API endpoint:

```yaml
profile_fragment:
  - type: api_fragment_render
  - route: fragments/profile
  - endpoint: https://api.example.test/profile
  - method: GET
  - template:
      file: profile.html
```

The endpoint is illustrative. If that API returns HTTP 200 with `{"name":"Alex"}`, the component renders `<p>Hello, Alex.</p>`. `.Data` contains the parsed API response; `.Status` contains the API’s HTTP status.

The browser response defaults to HTTP 200 independently of the API’s status. Use `.Status` in the template to decide which feedback to show. See [API Render](API_RENDER.md) for request mapping, response headers, and more examples.

## 9. Use one plugin for different actions

A form might let a user preview a result and then confirm it. Define a separate URL for each action. Both routes can use the same plugin, with a different `data.action` value.

This example uses the review plugin from the project lifecycle module:

```yaml
review_preview:
  - type: fragment
  - route: preview
  - result:
      - type: plugin
      - plugin: LifecycleTestPlugin__project-lifecycle-test@1.0.0
      - data:
          action: preview
          template:
            file: review-result.html

review_confirm:
  - type: fragment
  - route: confirm
  - result:
      - type: plugin
      - plugin: LifecycleTestPlugin__project-lifecycle-test@1.0.0
      - data:
          action: confirm
          template:
            file: review-result.html
```

`/preview` passes `preview` to the plugin; `/confirm` passes `confirm`. The plugin reads `data.action`, processes the submitted form, and returns values for the result template. YAML defines the URLs; the plugin implements the actions.

The plugin must be built and enabled, and its result template must be present. This demo previews and confirms a project name; it does not save it. See the [complete review plugin](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/project-lifecycle-test/plugins/lifecycle-test/1.0.0/lifecycle_test_plugin.go), [result template](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/project-lifecycle-test/profiles/plugin/templates/review-result.html), and [module instructions](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/project-lifecycle-test/README.md).

## 10. Protect a route with an access check

Some URLs should only be available to users with permission. For example, a settings page might require a user to sign in and have access to the selected project.

Add a `guard` to the `hypermedia`, `fragment`, or `api_fragment_render` component that defines the route. A guard checks the request on the server before the component renders. If access is denied, HyperBricks returns the configured denial response without rendering the content or running its plugins and API calls. You can configure a redirect to a login page or an HTTP error response.

A guard can read a token from a cookie or request header and send it to an authorization service to check permission. Finding a token is not the same as validating it. Hiding a link in the menu also does not prevent someone from requesting its URL directly.

This example protects a settings document:

```yaml
settings_page:
  - type: hypermedia
  - route: settings
  - title: Settings
  - nocache: true
  - guard:
      enabled: true
      auth:
        header: Authorization
        scheme: Bearer
      require:
        authenticated: true
      authorize:
        endpoint: https://auth.example.test/settings/access
        method: GET
      on_unauthenticated:
        default:
          status: 401
      on_forbidden:
        default:
          status: 403
  - content:
      - type: html
      - value: <h1>Settings</h1>
```

The authorization endpoint is illustrative. A request to `/settings` must include `Authorization: Bearer <token>`. HyperBricks forwards the token to the authorization service: a 2xx response allows rendering, 401 means unauthenticated, and 403 means forbidden. The configured denial responses return those statuses without the settings content.

`nocache: true` ensures each request runs the access check instead of reusing cached document output.

See [Route Guard](ROUTE_GUARD.md) for the YAML configuration and [the guarded page example](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/hyperbricks-patterns-yaml/docs/pages/guarded-page-demo.md) for a complete demonstration.

## 11. Export a static site

A static host serves generated HTML and assets without running HyperBricks.

Use a static export for content that can be rendered in advance. For a module named `demo`, this command exports the site and starts a local server to preview the generated files:

```bash
hyperbricks static -m demo --serve
```

The output is written to `modules/demo/rendered/`. Upload those files to a static host, such as GitHub Pages or Cloudflare Pages. Omit `--serve` if you only want to export the files.

Browser JavaScript still works, and HTMX can load exported fragments. Server-side calculations, access checks, and API actions need a running HyperBricks server; a static export only contains the results produced during the export. See [Static output](HYPERBRICKS_CLI.md#static-rendering) for export options.

For more complete examples, see the [patterns module source guide](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/hyperbricks-patterns-yaml/docs/SOURCE_GUIDE.md).

## 1. Enhance ordinary links with HTMX

Keep `href` pointed at the page a visitor can open directly. The enhancement can request a fragment and put the canonical page URL in browser history:

```html
<a href="/projects"
   hx-get="/fragments/projects"
   hx-target="#main-content"
   hx-swap="innerHTML"
   hx-push-url="/projects">Projects</a>
```

Here `#main-content` is the existing page container. This approach keeps a normal link useful when browser JavaScript is disabled. Verify Back, Forward, reload, active navigation, and page titles as part of the enhanced flow.

Another supported approach uses `menu` to request a canonical page and select the needed part of its HTML. See the existing [MENU example](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/hyperbricks-patterns-yaml/docs/pages/menu-htmx-demo.md). Choose the approach that fits the application; separate fragment routes are useful, but not a universal requirement for every navigation link.

## 2. Describe navigation in configuration

Put navigation labels, canonical paths, and fragment paths in a small data map and render that list through one shared template. A new section then needs one navigation entry instead of duplicated HTML in every page.

A shared navigation definition keeps labels and destinations together. A larger application can use route metadata with `menu` or configured sidebar entries with subsection links. The [sidebar navigation example](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/hyperbricks-patterns-yaml/docs/pages/sidebar-section-navigation.md) shows the latter. Keep role-based visibility separate from actual authorization.

**Try it:** add a navigation label and its route, then check both direct and HTMX navigation. The template should remain shared.

The example uses keyed maps for structured navigation and card collections. The current template value preprocessing treats sequences differently and can filter a direct list of maps. Use the verified map shape here, and keep numeric prefixes on keys when a particular display order is needed. When overriding a component inside a `values` map, repeat its `inherit` reference so the replaced value remains a complete component.

## References

- [Troubleshooting](TROUBLESHOOTING.md)
- [Migration Guide](MIGRATION.md)
- [HyperBricks Component Reference](REFERENCE.md)
- [HyperBricks YAML Usage](YAML_USAGE.md)
- [HyperBricks CLI](HYPERBRICKS_CLI.md)
- [Native esbuild](ESBUILD.md)
- [API Render](API_RENDER.md)
- [Plugins](PLUGINS.md)
- [Goja Render](GOJA_RENDER.md)
- [Routing](ROUTING.md)
- [Route Guard](ROUTE_GUARD.md)

## Module References
- [HyperBricks modules index](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/README.md)
- [hyperbricks-patterns-yaml](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/hyperbricks-patterns-yaml/README.md)
- [lifecycle_test_plugin.go](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/project-lifecycle-test/plugins/lifecycle-test/1.0.0/lifecycle_test_plugin.go)
- [review-result.html](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/project-lifecycle-test/profiles/plugin/templates/review-result.html)
- [Project lifecycle test fixture](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/project-lifecycle-test/README.md)
- [Guarded Page Demo](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/hyperbricks-patterns-yaml/docs/pages/guarded-page-demo.md)
- [Patterns module source guide](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/hyperbricks-patterns-yaml/docs/SOURCE_GUIDE.md)
- [MENU + HTMX Demo](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/hyperbricks-patterns-yaml/docs/pages/menu-htmx-demo.md)
- [Build sidebar navigation with section links](https://github.com/hyperbricks/hyperbricks/blob/v1.2.6-beta/modules/hyperbricks-patterns-yaml/docs/pages/sidebar-section-navigation.md)
