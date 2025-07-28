**Licence:** MIT  
**Version:** v0.5.99-alpha  
**Build time:** 2025-07-27T13:38:09Z


## Disclaimer

This project is a personal experiment, initially built for my own use. You’re welcome to use it however you like, but please be aware that it’s currently in an alpha stage and not recommended for production environments.

The project is released under the [MIT License](https://github.com/hyperbricks/hyperbricks/blob/main/LICENSE) and provided “as-is,” without any warranties or guarantees.

## HyperBricks Documentation

HyperBricks is a headless content management system that aims to bridge front and back-end development of [HTMX](https://htmx.org/) powered hypermedia applications.  

This is done by creating configuration files (referred to as "hyperbricks") that allows you to declare and describe the state of a document in a concise and structured manner.

### Key Features:
#### Declarative Configurations:
Write your web page structure, hierarchy and behavior in easy-to-read configuration files.
#### Dynamic Rendering: 
Use HTMX to dynamically update parts of your page without a full reload.
#### Modular and Reusable:
Configure components once and reuse them across pages through object inheritance and reusability.

Go direct to:
- [Quickstart](#quickstart)
- [Installation Instructions for HyperBricks](#installation-instructions-for-hyperbricks)
- [Defining Hypermedia Documents and Fragments](#defining-hypermedia-documents-and-fragments)
- [Adding Properties to Configurations](#adding-properties-to-configurations)
- [Rendering Order and Property Rules](#rendering-order-and-property-rules)
- [Example Configurations](#example-configurations)
  - [Hypermedia Example](#hypermedia-example)
  - [Fragment Example with HTMX Trigger](#fragment-example-with-htmx-trigger)
- [Object Inheritance and Reusability](#object-inheritance-and-reusability)
- [Importing Predefined HyperScripts](#importing-predefined-hyperscripts)
- [HyperBricks type reference](#hyperbricks-type-reference)
- [API Serverside Render](#api-serverside-render)

### Defining Hypermedia Documents and Fragments

##### Main Composite Configuration Types
HyperBricks organizes configuration files using a clear hierarchy:

`<HYPERMEDIA>`

Represents the primary configuration component for full-page documents in HyperBricks. It orchestrates page structure, including <head> and <body> sections, route handling, and overall document rendering.


`<TREE>`

The <TREE> type can nest items recursively, but it requires a <HYPERMEDIA> or <FRAGMENT> component as the root composite component.

`<FRAGMENT>`

Defines dynamically updateable sections of your pages, utilizing HTMX for efficient content updates without full-page reloads. Ideal for partial page rendering and interactivity. A `<FRAGMENT>` dynamically updates parts of an HTML page using HTMX, improving performance by avoiding full page reloads."

`<API_RENDER>`

Used for fetching and rendering external API data. Optimized for caching and public data consumption, typically nested within `<HYPERMEDIA>` or `<FRAGMENT>` components.

`<API_FRAGMENT_RENDER>`

Acts as a bi-directional proxy for API requests and responses, enabling secure and dynamic rendering of authenticated API data directly into HTMX fragments, managing authentication, JWT tokens, and session cookies.

`<TREE>`

Allows hierarchical nesting of components to structure complex content arrangements. Must be rooted within either a `<HYPERMEDIA>` or `<FRAGMENT>`.

`<TEMPLATE>`

Enables reusable HTML structures through Golang templates, promoting consistency and reducing redundancy across pages and fragments.


Certainly! Here are some concise and clear documentation examples illustrating how HyperBricks uses the standard Go `html/template` library extended with Sprig functions:

---

### HyperBricks Templates: Using Go html/template with Sprig

HyperBricks leverages the standard Go [html/template](https://pkg.go.dev/html/template) library, enriched by the powerful functions provided by [Sprig](https://masterminds.github.io/sprig/). This combination enables developers to write dynamic, reusable, and expressive templates for rendering HTML content within HyperBricks configurations.

---

## Simple Template Example

A basic template rendering dynamic data:

**Configuration:**
```properties
myTemplate = <TEMPLATE>
myTemplate {
    inline = <<[
        <h1>{{.title}}</h1>
        <p>{{.message}}</p>
    ]>>
    values {
        title = Hello World
        message = Welcome to HyperBricks!
    }
}
```

**Expected Result**
```html
<h1>Hello World</h1>
<p>Welcome to HyperBricks!</p>
```

---

### Iterating Data (Arrays)

Templates can iterate over data arrays easily:

```properties
fragment = <FRAGMENT>
fragment.route = get-users

fragment.10 = <TEMPLATE>
fragment.10 {
    inline = <<[
        <ul>
            {{$users := fromJson .users}}
            {{ range $users }}
                <li>{{.name}} ({{.email}})</li>
            {{ end }}
        </ul>
    ]>>
    values {
        # multiline string users
        users = <<[ 
            
            [
                { "name": "Alice", "email": "alice@example.com" },
                { "name": "Bob", "email": "bob@example.com" }
            ]

        ]>>
    }
}
```

**Expected Result**
```html
<ul>
    <li>Alice (alice@example.com)</li>
    <li>Bob (bob@example.com)</li>
</ul>
```

---

## Advanced Example: Using Sprig Functions

Sprig functions greatly expand the capabilities of templates. Here's how to leverage some useful Sprig functions within HyperBricks:

### Example with Sprig's `upper` and `date` Functions:

```properties
postTemplate = <TEMPLATE>
postTemplate {
    inline = <<[
        <article>
            <h2>{{ .title | upper }}</h2>
            <p>{{ .content }}</p>
            <footer>Published on {{ now | date "Jan 2, 2006" }}</footer>
        </article>
    ]>>
    values {
        title = Getting started
        content = HyperBricks makes it easy to build web applications with HTMX.
    }
}
```

**Expected Result**
```html
<article>
    <h2>GETTING STARTED</h2>
    <p> HyperBricks makes it easy to build web applications with HTMX.</p>
    <footer>Published on March 14, 2025</footer>
</article>
```

---

### Advanced Example with Sprig `default` and Conditional Logic

Template utilizing conditional logic and Sprig’s `default` function for robust rendering:

```properties
profileTemplate = <TEMPLATE>
profileTemplate {
    inline = <<[
        <div>
            <h3>{{ .username | default "Anonymous" }}</h2>
            {{ if .bio }}
                <p>{{ .bio }}</p>
            {{ else }}
                <p>No bio provided.</p>
            {{ end }}
        </div>
    ]>>
    values {
        title = Profile
        bio = 
    }
}
```

**Expected Result**
```html
<div>
    <h3>Anonymous</h2> 
    <p>No bio provided.</p>
</div>
```

---

### Helpful Resources:

- [Go html/template Official Documentation](https://pkg.go.dev/html/template)
- [Sprig Function Reference](https://masterminds.github.io/sprig/)

---

Hypermedia documents or fragments can be declared using simple key-value properties. This next example creates two locations on site root (index) and /somefragment

```properties
myHypermedia = <HYPERMEDIA>
myHypermedia.route = index

# Or
myFragment = <FRAGMENT>
myFragment.route = somefragment
```

### Adding Properties to Configurations

Add properties to hypermedia objects in either flat or nested formats

**Flat Configuration Example:**
```properties
fragment = <FRAGMENT>
fragment.route = myfragment
fragment.content = <TREE>
fragment.content.10 = <HTML>
fragment.content.10.value = <p>THIS IS HTML</p>
```

**Nested Configuration Example:**
```properties
fragment = <FRAGMENT>
fragment.route = myfragment
fragment {
    content = <TREE>
    content {
        10 = <HTML>
        10 {
            value = <p>THIS IS HTML</p>
        }
    }
}
```
`<FRAGMENT>` and `<API_FRAGMENT_RENDER>` declarations can contain response object keys. These are conform the HTMX documented headers.

**response header example:**

```properties
fragment = <FRAGMENT>
fragment.route = myfragment
fragment {
    content = <TREE>
    content {
        10 = <HTML>
        10 {
            value = <p>THIS IS HTML</p>
        }
    }
    response {
        hx_target = target-element-id
    }
}
```

Properties are rendered in alphanumeric order. Property values are typeless, so quotes are not required. Types such as ```<IMAGE>```, ```<HTML>```, or ```<TEXT>``` are identified automatically during parsing.

```properties
hypermedia = <HYPERMEDIA>
hypermedia.10 = <HTML>
hypermedia.10.value = <p>some text</p>

hypermedia.20 = <HTML>
hypermedia.20.value = <p>some more text</p>

hypermedia.1 = <HTML>
hypermedia.1 {
    value = <<[
        <p>RENDERS FIRST</p>
    ]>>
}
```

### Example Configurations

#### Hypermedia Example

A basic `<HYPERMEDIA>` object with nested `<IMAGE>` and `<TEXT>` types in a `<TEMPLATE>`:

```properties
hypermedia = <HYPERMEDIA>
hypermedia.route = index
hypermedia.head = <HEAD>
hypermedia.head {
    10 = <CSS>
    10.inline = <<[
        .content {
            color: green;
        }
    ]>>

    20 = <JAVASCRIPT>
    20.inline = <<[
        console.log("hello world");
    ]>>
    20.attributes {
        type = text/javascript
    }

    30 = <HTML>
    30.value = <<[
        <link rel="stylesheet" href="styles.css">
        <script src="main.js" type="text/javascript"></script>
    ]>>
}
hypermedia.10 = <TREE>
hypermedia.10 {
    1 = <HTML>
    1.value = <p>SOME CONTENT</p>
}
```

#### Fragment Example with HTMX Trigger

A `<FRAGMENT>` object using an [HTMX trigger](https://htmx.org/attributes/hx-trigger/) with nested `<IMAGE>` and `<TEXT>` types:

```properties
fragment = <FRAGMENT>
fragment.response {
    hx_trigger = myEvent
    hx_target = #target-element-id
}
fragment.10 = <TEMPLATE>
fragment.10 {
    template = <<[
        <h2>{{header}}</h2>
        <p>{{text}}</p>
        {{image}}
    ]>>
    istemplate = true
    values {
        header = SOME HEADER
        text = <TEXT>
        text.value = some text

        image = <IMAGE>
        image.src = hyperbricks-test-files/assets/cute_cat.jpg
        image.width = 800
    }
}
```

### Object Inheritance and Reusability

Properties can inherit from other objects. Here, `fragment.content.10` inherits from `myComponent`, with its `values.src` overridden:

```properties
myComponent = <TEMPLATE>
myComponent {
    template = <<[
        <iframe width="{{width}}" height="{{height}}" src="{{src}}"></iframe>
    ]>>
    istemplate = true
    values {
        width = 300
        height = 400
        src = https://www.youtube.com/embed/tgbNymZ7vqY
    }
}

fragment = <FRAGMENT>
fragment.content = <TREE>
fragment.content {
    10 < myComponent
    10.values.src = https://www.youtube.com/watch?v=Wlh6yFSJEms
    enclose = <div class="youtube_video">|</div>
}
```

### Importing Predefined HyperScripts

Predefined hyperscripts can be imported and reused:

```properties
#imports myComponent
@import "path/my_component.hyperbricks"

fragment = <FRAGMENT>
fragment.content = <TREE>
fragment.content {
    10 < myComponent
    10.values.src = https://www.youtube.com/watch?v=Wlh6yFSJEms
    enclose = <div class="youtube_video">|</div>
}
```
### **Fragment Example with Response Headers**
```properties
fragment = <FRAGMENT>
fragment.route = fragment_response
fragment {
    content = <TREE>
    content {
        10 = <HTML>
        10.value = <p>This is a fragment with response headers.</p>
    }
    response {
        hx_trigger = customEvent
        hx_target = #response-container
    }
}
```
This fragment is triggered on the client side by `customEvent`, updating the content in the DOM element with the ID `#response-container`.
---

### **Hypermedia with Template**
```properties
hypermedia = <HYPERMEDIA>
hypermedia.route = template_page
hypermedia.title = Template Example
hypermedia.10 = <TEMPLATE>
hypermedia.10 {
    inline = <<[
        <h1>{{title}}</h1>
        <p>{{content}}</p>
    ]>>
    values {
        title = Welcome!
        content = This is a template-driven hypermedia page.
    }
}
```
This Hypermedia uses a template to structure its content dynamically.

---

### **Hypermedia with Multiple Ordered Items**
```properties
hypermedia = <HYPERMEDIA>
hypermedia.route = ordered_content
hypermedia.title = Ordered Items
hypermedia.10 = <HTML>
hypermedia.10.value = <p>Item 10</p>

hypermedia.20 = <HTML>
hypermedia.20.value = <p>Item 20</p>

hypermedia.30 = <HTML>
hypermedia.30.value = <p>Item 30</p>
```
Content is ordered numerically and renders in that sequence.

---

### **API Render Example**
```properties
api_render = <API_RENDER>
api_render.route = api_example
api_render.url = https://api.example.com/data
api_render.method = GET
api_render.inline = <<[
    <h1>{{.title}}</h1>
    <p>{{.description}}</p>
]>>
```
Fetches data from an API and renders it using a template.

---

### **API Fragment Render Example**
```properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment.route = api_fragment_example
api_fragment.url = https://api.example.com/fragment
api_fragment.method = POST
api_fragment.inline = <<[
    <div>{{.content}}</div>
]>>
api_fragment.response {
    hx_target = #fragment-container
    hx_trigger = newData
}
```
This API fragment fetches data and dynamically updates `#fragment-container`.

---

### **Image Example**
```properties
hypermedia = <HYPERMEDIA>
hypermedia.route = image_example
hypermedia.title = Image Display
hypermedia.10 = <IMAGE>
hypermedia.10 {
    src = https://picsum.photos/400
    width = 400
    height = 300
    alt = Random Image
}
```
Loads a placeholder image dynamically.



```properties
# Define the main Hypermedia document
hypermedia = <HYPERMEDIA>

# The route determines the URL path for this Hypermedia document (e.g., "/index")
hypermedia.route = index

# Title of the page (used in the document title and as a variable in the template)
hypermedia.title = Structured Page

# Defines the <body> tag attributes, such as a background color and padding
# The "|" character separates the opening and closing tag
hypermedia.bodytag = <body class="bg-gray-100 p-4">|</body>

# The <HEAD> section of the document, automatically available in <HYPERMEDIA>
# This can contain meta tags, stylesheets, and scripts
hypermedia.head {
    # Assigning priority 100 to ensure this block loads properly
    100 = <HTML>
    100.value = <<[
        <!-- Meta tags for character encoding and viewport settings -->
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">

        <!-- Internal CSS for basic styling -->
        <style>
            body { font-family: Arial, sans-serif; margin: 20px; }
            header, footer { background: #333; color: white; padding: 10px; text-align: center; }
            main { padding: 20px; }
        </style>

        <!-- External CSS: TailwindCSS for modern utility-based styling -->
        <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/tailwindcss@2.2.19/dist/tailwind.min.css">

        <!-- External JavaScript: HTMX for handling dynamic updates without full-page reloads -->
        <script src="https://unpkg.com/htmx.org@2.0.4"></script>
    ]>>
}

# The template is already predefined in <HYPERMEDIA>
# It determines the full structure of the HTML document and dynamically injects content
hypermedia.template {
    inline = <<[
        <!DOCTYPE html>
        <html lang="en">
        
        <!-- The head section is injected dynamically by hyperbricks using the head marker, 
             which pulls from hypermedia.head (not from values object) -->

        {{.head}}

        <body>
            <header>
                <!-- Injects the title dynamically from hypermedia.template.values.title -->
                <h1>{{.title}}</h1>
            </header>
            <main>
                <!-- Injects dynamic content from hypermedia.values.content -->
                <p>{{.content}}</p>
            </main>
            <footer>
                <p>&copy; 2025 My Website</p>
            </footer>
        </body>
        </html>
    ]>>

    # Predefined values injected into the template
    values {
        # This is referenced in the template with {{.title}}
        title = Structured Page
          
        # Used in {{.content}}
        content = This is a Hypermedia document with a full HTML structure.  
    }
}
```
html result:
```html
<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
        }

        header,
        footer {
            background: #333;
            color: white;
            padding: 10px;
            text-align: center;
        }

        main {
            padding: 20px;
        }
    </style>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/tailwindcss@2.2.19/dist/tailwind.min.css">
    <script src="https://unpkg.com/htmx.org@2.0.4"></script>
    <meta name="generator" content="hyperbricks cms">
    <title>Structured Page</title>
</head>

<body>
    <header>
        <h1>Structured Page</h1>
    </header>
    <main>
        <p>This is a Hypermedia document with a full HTML structure.</p>
    </main>
    <footer>
        <p>&copy; 2025 My Website</p>
    </footer>
</body>

</html>
```
## Hyperbricks Configuration Reference

This section explains the configuration options for your `package.hyperbricks` file.

---

### Module Declaration

```ini
# Set the current module path
$module = modules/default
```

Use the `$module` variable to reference the module directory throughout your configuration.

---

### Global Configuration Objects

You can define global config blocks (e.g., for custom use):

```ini
myconf {
    some = value
}
```

Only the `hyperbricks` object is processed by the runtime. Other objects are allowed for organizational or user-defined purposes.

---

### Main Configuration Block

#### `hyperbricks { ... }`

This is the primary configuration block.

---

### Mode Settings

```ini
mode = development
```

Available modes:

* `development` – Ideal for local dev with live reload.
* `live` – Optimized for production deployment.
* `debug` – Extra verbose logging (used for diagnostics).

---

### Debugging

```ini
debug {
    level = debugging
}
```

Controls Go-level debug verbosity.

---

### Development Mode

```ini
development {
    watch = true
    reload = true
    frontend_errors = false
    dashboard = false
}
```

These settings are active only in `development` mode.

---

### Live Mode

```ini
live {
    cache = 10s
}
```

* Sets cache duration for rendered pages.
* Supports Go-style durations like `300ms`, `2h45m`, etc.

---

### Server Settings

```ini
server {
    port = 8080
    beautify = true
    read_timeout = 5s
    write_timeout = 10s
    idle_timeout = 20s
}
```

Adjust timeout values based on traffic level. High-traffic blogs may prefer higher values (see commented examples).

---

### System Settings

```ini
system {
    metrics_watch_interval = 10s
}
```

Interval for system ticker to gather and report metrics.

---

### Rate Limiting

```ini
rate_limit {
    requests_per_second = 100
    burst = 500
}
```

Control traffic with configurable request and burst limits. Adjust for your traffic level.

---

### Plugins

```ini
plugins {
    # Example:
    # enabled = [ plugin.so , otherplugin.so ]
}
```

Enable plugins by listing their exact `.so` filenames. Use `hyperbricks plugin help` and `hyperbricks plugin list` for details.
See plugin section on how to create and/or install plugins for Hyperbricks.

---

### Directory Settings

```ini
directories {
    render      = {{VAR:module}}/rendered
    static      = {{VAR:module}}/static
    resources   = {{VAR:module}}/resources
    plugins     = ./bin/plugins/
    templates   = {{VAR:module}}/templates
    hyperbricks = {{VAR:module}}/hyperbricks
    # logs = {{VAR:module}}/logs
}
```

| Key           | Purpose                                                 |
| ------------- | ------------------------------------------------------- |
| `render`      | Where static output is generated (`hyperbricks static`) |
| `static`      | Public assets served as-is, like minified JS/CSS        |
| `resources`   | Raw, unprocessed files like JS sources or markdown      |
| `plugins`     | Path to `.so` plugin files                              |
| `templates`   | Used with `<TEMPLATE>` and template engines             |
| `hyperbricks` | Directory to scan for `.hyperbricks` files              |
| `logs`        | (Optional) Enable file-based logging                    |

---

## Module Directory Structure Guide

A Hyperbricks module follows a specific folder structure. Each folder serves a unique role in how your module is configured, rendered, and served.

```
someproject/
├── hyperbricks/
├── rendered/
├── resources/
├── static/
├── templates/
└── package.hyperbricks
```

---

### Folder Breakdown

#### `hyperbricks/`

Contains the core `.hyperbricks` configuration files.

* Files are loaded automatically in alphanumeric order from the root of this folder.
* Subdirectories are not auto-loaded — you must explicitly include them using `@import`.

---

#### `rendered/`

Default output folder used by the `hyperbricks static` command.

* Stores pre-rendered routes and other static outputs.
* Contents are typically generated — not edited by hand.

---

#### `resources/`

Raw asset and source data folder.

Use this to store:

* JavaScript source files
* Tailwind or other build configurations
* Uncompiled markdown documents
* Unprocessed images or data files

---

#### `static/`

Public asset directory available at runtime.

* Served relative to the root domain.
* Example: `static/css/styles.min.css` becomes `https://mydomain.com/static/css/styles.min.css`

Use it for precompiled CSS, JS, fonts, or images.

---

#### `templates/`

Stores template files used during rendering.

* Used for `<TEMPLATE>` blocks and any other templated output components.
* Often embedded into config via `hypermedia` markers.

---

#### `package.hyperbricks`

Module entry point.

* Defines the module's main configuration.
* Links together scripts, templates, resources, and routes.

---

### Path Markers in Configurations

Use these markers in your configuration files for cleaner, portable paths:

| Marker        | Refers To                  |
| ------------- | -------------------------- |
| `MODULE`      | Current module directory   |
| `MODULE_ROOT` | Root folder of all modules |
| `RESOURCES`   | `resources/` directory     |
| `HYPERBRICKS` | `hyperbricks/` directory   |
| `TEMPLATES`   | `templates/` directory     |
| `STATIC`      | `static/` directory        |
| `ROOT`        | Root of the entire project |

---

## Hypermedia Markers: Cached Content Embedding

The `hypermedia` marker lets you preload files into memory and assign them as templates or raw text blocks. These are injected into the configuration and made instantly available — no runtime file access required.

### Why use it?

* Fast rendering
* Self-contained config
* Immutable state after load

### Structure

```
hypermedia.<key> = <TYPE>
hypermedia.<key>.<field> = {{<TYPE>:<file>}}
```

| Type       | Field      | Loaded From                |
| ---------- | ---------- | -------------------------- |
| `TEMPLATE` | `template` | `templates/` in the module |
| `TEXT`     | `value`    | Module's root directory    |

### Examples

```ini
hypermedia.10 = TEMPLATE
hypermedia.10.template = {{TEMPLATE:sometemplate.html}}

hypermedia.10 = TEXT
hypermedia.10.value = {{TEXT:sometext.md}}
```

---

## Quickstart

### 1. Install Hyperbricks

Requirements:

* Go 1.23.2 or higher

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
```

---

### 2. Initialize a New Project

```bash
hyperbricks init -m someproject
```

This creates a folder `someproject` in the `modules` directory with this structure:

```
someproject/
├── hyperbricks/
├── rendered/
├── resources/
├── static/
├── templates/
└── package.hyperbricks
```

Always run Hyperbricks CLI from the **project root** (parent of `modules/`).

---

### 3. Start the Project

```bash
hyperbricks start -m someproject
```

Visit: [http://localhost:8080](http://localhost:8080)

To see CLI options:

```bash
hyperbricks start --help
```

---

### 4. Render Static Output

```bash
hyperbricks static -m someproject
```

---

### Other Commands

```bash
hyperbricks [command]
```

| Command      | Description                                |
| ------------ | ------------------------------------------ |
| `completion` | Generate shell autocompletion              |
| `help`       | Help on any command                        |
| `init`       | Create config and folders for a new module |
| `select`     | Select the active module                   |
| `plugin`     | plugin management commands                 |
| `start`      | Start the server                           |
| `static`     | Render static HTML output                  |
| `version`    | Show version info                          |

Use `hyperbricks [command] --help` for detailed options.

## Hyperbricks Plugin System

Hyperbricks supports plugins compiled as Go `.so` files. The CLI offers tools to discover, install, build, and manage plugins compatible with your current version of Hyperbricks.

---

### Overview

To access plugin management commands:

```bash
hyperbricks plugin [subcommand]
```

Subcommands:

| Command          | Description                                           |
| ---------------- | ----------------------------------------------------- |
| `plugin list`    | List compatible plugins and their installation status |
| `plugin install` | Download and compile a plugin by name and version     |
| `plugin build`   | Rebuild a plugin from local source                    |
| `plugin remove`  | Remove a compiled plugin from the local system        |
| `plugin update`  | Update a plugin to the latest compatible version      |

---

## `plugin list`

Displays available plugins that are compatible with your installed Hyperbricks version.

```bash
hyperbricks plugin list
```

Output includes:

* Plugin name
* Compatible version
* All available versions
* Hyperbricks version constraints
* Installation status (e.g. installed, incompatible, not found)

Example output:
```
Name         Plugin Version  Available Versions  Compatible Hyperbricks  Installed
----         --------------  ------------------  ----------------------  ---------
esbuild      1.0.0           1.0.0               >=0.5.0-alpha           yes
loremipsum   1.0.0           1.0.0               >=0.5.0-alpha           yes
markdown     1.0.0           1.0.0               >=0.5.0-alpha           yes
myplugin     1.0.0           1.0.0               >=0.5.0-alpha           no
tailwindcss  1.0.0           1.0.0               >=0.5.0-alpha           yes
```

To enable plugins, they must be compiled for the currently installed version of Hyperbricks.
This can be done automatically using:
 hyperbricks plugin install <name>@<plugin_version> 

* To preload the plugin, add the binary .so name to your package.hyperbricks
* under the `plugins.enabled` array:
* Plugin binaries are named as <name>@<plugin_version>.so for clarity.
```
plugins {
  enabled = [ EsbuildPlugin@1.0.0.so, LoremIpsumPlugin@1.0.0.so, MarkdownPlugin@1.0.0.so, TailwindcssPlugin@1.0.0.so ]
}
```
---

## `plugin install <name>[@<version>]`

Installs and builds a plugin from the remote Hyperbricks plugin index.

```bash
hyperbricks plugin install markdown@1.0.0
```

If no version is specified, the latest compatible version is used.

### What it does:

1. **Fetches metadata** from the remote plugin index.
2. **Performs a sparse Git checkout** of the plugin's source files.
3. **Patches the plugin’s `go.mod`** to match your current Hyperbricks version.
4. **Runs `go mod tidy` and `go build`** to compile the plugin into `./bin/plugins`.

---

## `plugin build <name>@<version>`

Builds a plugin from local source already downloaded to `./plugins/<name>/<version>`.

```bash
hyperbricks plugin build markdown@1.0.0
```

Useful when:

* You manually edited a plugin
* You cloned or fetched plugin sources yourself

---

## `plugin remove <name>@<version>`

Removes a compiled `.so` plugin binary from `./bin/plugins`.

```bash
hyperbricks plugin remove markdown@1.0.0
```

This does not delete the source folder from `./plugins`.

---

## `plugin update <name>`

(Planned) Checks for and installs the latest compatible version of a given plugin.

```bash
hyperbricks plugin update markdown
```

*Note: This command currently shows placeholder output. Update logic is not yet implemented.*

---

## Using Installed Plugins

To enable a plugin in your module, add its `.so` filename to the `plugins.enabled` list in `package.hyperbricks`.

Example:

```ini
plugins {
    enabled = [ Markdown@1.0.0.so ]
}
```

> Plugin binary filenames follow the format `<CamelCaseName>@<version>.so`.

---

## Plugin Compatibility

* A plugin is only valid if compiled for the same version of Hyperbricks as you're running.
* The CLI inspects `.so` binaries to validate their embedded version.
* Incompatible or outdated plugins are flagged in yellow or red when using `plugin list`.


<h1><a id="hyperbricks-type-reference">HyperBricks type reference</a></h1>

### Component categories:
 

### **component**

- [&lt;HTML&gt;](#<HTML>) 
- [&lt;TEXT&gt;](#<TEXT>) 


### **composite**

- [&lt;FRAGMENT&gt;](#<FRAGMENT>) 
- [&lt;HEAD&gt;](#<HEAD>) 
- [&lt;HYPERMEDIA&gt;](#<HYPERMEDIA>) 
- [&lt;TEMPLATE&gt;](#<TEMPLATE>) 
- [&lt;TREE&gt;](#<TREE>) 


### **data**

- [&lt;JSON&gt;](#<JSON>) 


### **menu**

- [&lt;MENU&gt;](#<MENU>) 


### **resources**

- [&lt;CSS&gt;](#<CSS>) 
- [&lt;IMAGE&gt;](#<IMAGE>) 
- [&lt;IMAGES&gt;](#<IMAGES>) 
- [&lt;JS&gt;](#<JS>) 





### Category: **component**





<h3><a id="&lt;HTML&gt;">&lt;HTML&gt;</a></h3>

**Type Description**










**Properties**

- [enclose](#html-enclose)
- [value](#html-value)
- [trimspace](#html-trimspace)





## html enclose
#### enclose

**Description**  
The enclosing HTML element for the header divided by |


**Example**
````properties
html = <HTML>
html.value = <<[
        <p>HTML TEST</p>    
    ]>>
html.enclose = <div>|</div>
}

````

**Expected Result**

````html
<div>
  <p>
    HTML TEST
  </p>
</div>
````












## html value
#### value

**Description**  
The raw HTML content


**Example**
````properties
html = <HTML>
html.value = <p>HTML TEST</p>    
}

````

**Expected Result**

````html
<p>
  HTML TEST
</p>
````












## html trimspace
#### trimspace

**Description**  
Property trimspace filters (if set to true true),  all leading and trailing white space removed, as defined by Unicode.


**Example**
````properties
html = <HTML>
html.value = <<[
        <p>HTML TEST</p>    
    ]>>
html.trimspace = true
}

````

**Expected Result**

````html
<p>
  HTML TEST
</p>
````













<h3><a id="&lt;TEXT&gt;">&lt;TEXT&gt;</a></h3>

**Type Description**








**Properties**

- [enclose](#text-enclose)
- [value](#text-value)





## text enclose
#### enclose

**Description**  
The enclosing HTML element for the text divided by |


**Example**
````properties
text = <TEXT>
text {
	  value = SOME VALUE
    enclose = <span>|</span>
}

````

**Expected Result**

````html
<span>
  SOME VALUE
</span>
````












## text value
#### value

**Description**  
The paragraph content


**Example**
````properties
text = <TEXT>
text {
	value = SOME VALUE
    enclose = <span>|</span>
}

````

**Expected Result**

````html
<span>
  SOME VALUE
</span>
````












### Category: **composite**




   ##  `<API_RENDER>` & `<API_FRAGMENT_RENDER>`

### API Serverside Render

The API components acts like a bi-directional PROXY that renders response data into HTMX-compatible responses, including HTMX response headers when using <API_FRAGMENT_RENDER>.

API call with json body
```properties
    body = <<[
        {
            "username":"emilys",
            "password":"emilyspass",
            "expiresInMins":30
        }
    ]>> 
```
The data can be mapped from form or body POST data. Use $ symbol to map the specific value like this:

```properties
    body = <<[
        {
            "username":"$form_username",
            "password":"$password"
        }
    ]>> 
```

## Data structure of available data for templating:
```go
// in case of an array or object, Values is always in root and use Data to access response data...
	struct {
		Data   interface{} // Can be anything
		Values map[string]interface{} // define this in values field
		Status int // the Status of the API response
	}
```

### `<API_FRAGMENT_RENDER>`

- Use with custom route
- Renders API Fetched Data to HTMX fragments
- Acts like bi-directional PROXY
- Validates headers and filters query params
- Maps Client Body data to Body of API request
- Handles JWT-based and Basic authentication
- Includes `jwtsecret` and `jwtclaims` options
- Uses cookies for session-based auth if needed
- Can respond with HTMX response headers
- Custom headers

### `<API_RENDER>`
- Is nested and optional cached, so it needs a parent composite component like `<FRAGMENT>` or `<HYPERMEDIA>`
- Renders Fetched Data to HTMX output, based on config values.
- Is Cached, depending on Hypermedia's configuration
- Passes API requests through, modifies headers, filters query params.
- Handles JWT-based and Basic authentication before making API requests.
- Uses cookies for session-based auth if needed.

### **Key Differences Between `<API_RENDER>` and `<API_FRAGMENT_RENDER>` Mode**
| Feature              | `<API_RENDER>` | `<API_FRAGMENT_RENDER>` |
|----------------------|-----------------------------|-----------------------------|
| **Cache** | ✅ Yes (optional)| ❌ No (explicit)|
| **API Request** | ✅ Yes | ✅ Yes |
| **Query Param Filtering (`querykeys`)** | ✅ Yes | ✅ Yes |
| **Custom Headers** | ✅ Yes | ✅ Yes |
| **Request Body Modification** | ✅ Yes | ✅ Yes |
| **Transforms Response (`inline`/`template`)** | ✅ Yes | ✅ Yes |
| **Debugging (`debug = true`)** | ✅ Yes | ✅ Yes |

### **Client->Server Interaction**
`<API_RENDER>` does not handle specific user auth. That makes this component only suited for fetching and rendering public data that can be cached on a interval. This can be set in the root composite component.

`<API_FRAGMENT_RENDER>` Can handle Client auth requests based on login forms and tokens that will be passed through bi-directional.
| `Client->Server` | `<API_RENDER>` | `<API_FRAGMENT_RENDER>` |
|----------------------|-----------------------------|-----------------------------|
| **Client->Server: JWT Authentication (`jwtsecret`)** | ❌ No | ✅ Yes |
| **Client->Server: Session-Based Auth (Cookies)** | ❌ No | ✅ Yes |
| **Client->Server: Basic Auth username and password** |❌ No  | ✅ Yes |
| **Client->Server: Generates JWT with Claims (`jwtclaims`)** | ❌ No | ✅ Yes |
| **Client->Server: Body and formdata mapping** | ✅ Yes (for public API, non-cached) | ✅ Yes |

### **Server->API Interaction**
Both components can apply authentication on API requests. So for example a Weather Service that requires a 
API key can be set by adding a header or by creating a JWT claim based on a secret
| `Server->API` | `<API_RENDER>` | `<API_FRAGMENT_RENDER>` |
|----------------------|-----------------------------|-----------------------------|
| **Server->API: JWT Authentication (`jwtsecret`)** |✅ Yes  | ✅ Yes |
| **Server->API: Session-Based Auth (Cookies)** | ✅ Yes  | ✅ Yes |
| **Server->API: Basic Auth username and password** |✅ Yes   | ✅ Yes |
| **Server->API: Generates JWT with Claims (`jwtclaims`)** | ✅ Yes  | ✅ Yes |


### **Other Interactions**
| `Server->API` | `<API_RENDER>` | `<API_FRAGMENT_RENDER>` |
|----------------------|-----------------------------|-----------------------------|
| **API->Server: Proxy Cookies (`setcookie`)** | ❌ No (set cookie headers for API request if required) | ✅ (acts like proxy) |
| **Server->Client: Sets Cookies (`setcookie`)** | ❌ No | ✅ Yes |


[See HTMX response header documentation](https://htmx.org/reference/#response_headers)

#### HTMX Response Headers for `<API_FRAGMENT_RENDER>`

This document provides an overview of the HTML headers used in the `HxResponse` struct, their corresponding mapstructure keys, and their descriptions.

| Hyperbricks Key              | HTMX Header                 | Description |
|-------------------------------|-----------------------------|-------------|
| hx_location                   | HX-Location                 | Allows you to do a client-side redirect that does not do a full page reload |
| hx_push_url                   | HX-Push-Url               | Pushes a new URL into the history stack |
| hx_redirect                   | HX-Redirect                 | Can be used to do a client-side redirect to a new location |
| hx_refresh                    | HX-Refresh                  | If set to &#39;true&#39; the client-side will do a full refresh of the page |
| hx_replace_url                | HX-Replace-URL              | Replaces the current URL in the location bar |
| hx_reswap                     | HX-Reswap                   | Allows you to specify how the response will be swapped |
| hx_retarget                   | HX-Retarget                 | A CSS selector that updates the target of the content update |
| hx_reselect                   | HX-Reselect                 | A CSS selector that allows you to choose which part of the response is used to be swapped in |
| hx_trigger                    | HX-Trigger                  | Allows you to trigger client-side events |
| hx_trigger_after_settle        | HX-Trigger-After-Settle     | Allows you to trigger client-side events after the settle step |
| hx_trigger_after_swap          | HX-Trigger-After-Swap       | Allows you to trigger client-side events after the swap step |


## <API_FRAGMENT_RENDER> examples

#### example 1
This is a login example via json body. After the request, the client cookie is set with setcookie field by applying template marker.

```properties

# Login with auth via body json and set returned token as cookie in the client's browser
api_login = <API_FRAGMENT_RENDER>
api_login {
    # this is the fragment route:
    route = login
    endpoint = https://dummyjson.com/auth/login
	method = POST

	# use body...
    body = {"username":"emilys","password":"emilyspass","expiresInMins":30}

    # https://dummyjson.com does not have basic auth option but basic auth can be set like this:
	# username = emilys
	# password = emilyspass

	headers {
		Access-Control-Allow-Credentials = true
		Content-Type = application/json
	}
	 inline = <<[
        {{ if eq .Status 200 }}
            <h1>{{.Values.someproperty}}</h1>
            <ul id="{{index .Data.id}}">
                <li>{{index .Data.firstName}} {{index .Data.lastName}}</li>
                <img src="{{index .Data.image}}">
            <ul>

            {{ else }} 
                {{.Data.message}}
        {{ end }}
	 ]>>
     values {
         someproperty = API_FRAGMENT_RENDER demo
     }
    debug = true
   
    # this is the template for setting the token (accessToken)
    setcookie =  <<[token={{.Data.accessToken}}]>>
    # response data is always found in .Data
}
```

### expected output example 1
```html
<h1>API_FRAGMENT_RENDER demo</h1>
<ul id="1">
    <li>Emily Johnson</li>
    <img src="https://dummyjson.com/icon/emilys/128">
<ul>

```

#### example 2
The client has cookie token set and passed by the component like for example:

`Authorization = Bearer <replace_token_here>`

```properties
api_me_render = <API_FRAGMENT_RENDER>
api_me_render {
    route = me
    endpoint = https://dummyjson.com/auth/me
	method = GET
	headers {
        # this can be commented out when using a browser because Authorization header is set by the previous example
        # Authorization = Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwidXNlcm5hbWUiOiJlbWlseXMiLCJlbWFpbCI6ImVtaWx5LmpvaG5zb25AeC5kdW1teWpzb24uY29tIiwiZmlyc3ROYW1lIjoiRW1pbHkiLCJsYXN0TmFtZSI6IkpvaG5zb24iLCJnZW5kZXIiOiJmZW1hbGUiLCJpbWFnZSI6Imh0dHBzOi8vZHVtbXlqc29uLmNvbS9pY29uL2VtaWx5cy8xMjgiLCJpYXQiOjE3NDE3Nzk0MTQsImV4cCI6MTc0MTc4MTIxNH0.VsZFlDJg5rtbau0v7QVNKRZifPBIK-s9R_6QuYpSxwY
        #Access-Control-Allow-Credentials = true
		#Content-Type = application/json
        #Accept = application/json
	}
	 inline = <<[
        {{ if eq .Status 200 }}
            <h1>{{.Values.someproperty}}</h1>
            <ul id="{{index .Data.id}}">gender
                <li>{{index .Data.firstName}} {{index .Data.lastName}}</li>
                <li>gender: {{index .Data.gender}} </li>
                <li>Bank CardNumber: {{index .Data.bank.cardNumber}} </li>
            <img src="{{index .Data.image}}">
            <ul>

            {{ else }} 
                {{.Data.message}}
        {{ end }}
        
	 ]>>
     values {
         someproperty = API_FRAGMENT_RENDER demo
     }

    
	enclose = <div class="user">|</div>
}

```
### expected output example 2
```html
<div class="user">
<h1>API_FRAGMENT_RENDER demo</h1>
    <ul id="1">gender
        <li>Emily Johnson</li>
        <li>gender: female </li>
        <li>Bank CardNumber: 9289760655481815 </li>
        <img src="https://dummyjson.com/icon/emilys/128">
    <ul>
</div>
```


<h3><a id="&lt;FRAGMENT&gt;">&lt;FRAGMENT&gt;</a></h3>

**Type Description**






A FRAGMENT dynamically renders a part of an HTML page, allowing updates without a full page reload and improving performance and user experience.


**Main Example**
````properties
fragment = <FRAGMENT>
fragment.response.hx_trigger = myEvent
fragment.10 = <TEMPLATE>
fragment.10 {
    inline = <<[
        <h2>{{.header}}</h2>
        <p>{{.text}}</p>
        {{.image}}
]>>
    
    values {
        header = SOME HEADER
        text = <TEXT>
        text.value = some text

        image = <IMAGE>
        image.src = hyperbricks-test-files/assets/cute_cat.jpg
        image.width = 800
    }
}

````


**Expected Result**
````html
<h2>
  SOME HEADER
</h2>
<p>
  some text
</p>
<img src="static/images/cute_cat_w800_h800.jpg" width="800" height="800" />
````


**more**


















**Properties**

- [response](#fragment-response)

- [title](#fragment-title)
- [route](#fragment-route)
- [section](#fragment-section)
- [enclose](#fragment-enclose)
- [index](#fragment-index)
- [content_type](#fragment-content_type)





## fragment response
#### response

**Description**  
HTMX response header configuration.


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_trigger = trigger-element-id
    }
}

````


[See HTMX response header documentation](https://htmx.org/reference/#response_headers)

## HTMX Response Headers

This document provides an overview of the HTML headers used in the `HxResponse` struct, their corresponding mapstructure keys, and their descriptions.

| Hyperbricks Key              | HTMX Header                 | Description |
|-------------------------------|-----------------------------|-------------|
| hx_location                   | HX-Location                 | Allows you to do a client-side redirect that does not do a full page reload |
| hx_push_url                   | HX-Push-Url               | Pushes a new URL into the history stack |
| hx_redirect                   | HX-Redirect                 | Can be used to do a client-side redirect to a new location |
| hx_refresh                    | HX-Refresh                  | If set to &#39;true&#39; the client-side will do a full refresh of the page |
| hx_replace_url                | HX-Replace-URL              | Replaces the current URL in the location bar |
| hx_reswap                     | HX-Reswap                   | Allows you to specify how the response will be swapped |
| hx_retarget                   | HX-Retarget                 | A CSS selector that updates the target of the content update |
| hx_reselect                   | HX-Reselect                 | A CSS selector that allows you to choose which part of the response is used to be swapped in |
| hx_trigger                    | HX-Trigger                  | Allows you to trigger client-side events |
| hx_trigger_after_settle        | HX-Trigger-After-Settle     | Allows you to trigger client-side events after the settle step |
| hx_trigger_after_swap          | HX-Trigger-After-Swap       | Allows you to trigger client-side events after the swap step |











## fragment title
#### title

**Description**  
The title of the fragment, only used in the context of the &lt;MENU&gt; component. For document title use &lt;HYPERMEDIA&gt; type.


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	title = Some Title
}

````










## fragment route
#### route

**Description**  
The route (URL-friendly identifier) for the fragment


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	route = index
}

````










## fragment section
#### section

**Description**  
The section the fragment belongs to. This can be used with the component &lt;MENU&gt; for example.


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	section = some_section
}

````










## fragment enclose
#### enclose

**Description**  
Enclosing property using the pipe symbol |


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	10 = <HTML>
    10.value = <p>TEST HTML</p>
    enclose = <div>|</div>
}

````

**Expected Result**

````html
<div>
  <p>
    TEST HTML
  </p>
</div>
````












## fragment index
#### index

**Description**  
Index number is a sort order option for the &lt;MENU&gt; section. See &lt;MENU&gt; for further explanation


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	index = 1
}

````










## fragment content_type
#### content_type

**Description**  
content type header definition


**Example**
````properties
fragment = <FRAGMENT>
fragment.content_type = text/json 

````











<h3><a id="&lt;HEAD&gt;">&lt;HEAD&gt;</a></h3>

**Type Description**




**Properties**







<h3><a id="&lt;HYPERMEDIA&gt;">&lt;HYPERMEDIA&gt;</a></h3>

**Type Description**








**Properties**

- [index](#hypermedia-index)
- [content_type](#hypermedia-content_type)





## hypermedia index
#### index

**Description**  
Index number is a sort order option for the hypermedia defined in the section field. See &lt;MENU&gt; for further explanation and field options


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia {
	index = 1
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <body></body>
</html>
````












## hypermedia content_type
#### content_type

**Description**  
content type header definition


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia {
	
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <body></body>
</html>
````













<h3><a id="&lt;TEMPLATE&gt;">&lt;TEMPLATE&gt;</a></h3>

**Type Description**










**Properties**

- [querykeys](#template-querykeys)
- [queryparams](#template-queryparams)
- [enclose](#template-enclose)





## template querykeys
#### querykeys

**Description**  
The inline template used for rendering.


**Example**
````properties
myComponent = <TEMPLATE>
myComponent {
    
    queryparams = {
        somequeryparameter = helloworld
    }
    querykeys = [somequeryparameter]

   
    values {
        width = 300
        height = 400
        src = https://www.youtube.com/embed/tgbNymZ7vqY
    }
}


````










## template queryparams
#### queryparams

**Description**  
The inline template used for rendering.


**Example**
````properties
myComponent = <TEMPLATE>
myComponent {
    
    queryparams = {
        somequeryparameter = helloworld
    }
    querykeys = [somequeryparameter]

   
    values {
        width = 300
        height = 400
        src = https://www.youtube.com/embed/tgbNymZ7vqY
    }
}


````










## template enclose
#### enclose

**Description**  
Enclosing property for the template rendered output divided by |


**Example**
````properties
myComponent = <TEMPLATE>
myComponent {
    inline = <<[
      <img src="{{.src}}" alt="{{.alt}}" width="{{.width}}" height="{{.height}}">
    ]>>
    values {
        width = 500
        height = 600
        alt = Girl in a jacket
        src = img_girl.jpg
    }
    enclose = <div id="image-container">|</div>
}

````

**Expected Result**

````html
<div id="image-container">
  <img src="img_girl.jpg" alt="Girl in a jacket" width="500" height="600">
</div>
````













<h3><a id="&lt;TREE&gt;">&lt;TREE&gt;</a></h3>

**Type Description**




**Properties**






### Category: **data**





<h3><a id="&lt;JSON&gt;">&lt;JSON&gt;</a></h3>

**Type Description**


















**Properties**

- [attributes](#json-attributes)
- [enclose](#json-enclose)
- [file](#json-file)
- [template](#json-template)
- [inline](#json-inline)
- [values](#json-values)
- [debug](#json-debug)





## json attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action


**Example**
````properties
local_json_test = <JSON_RENDER>
local_json_test {
	file =  hyperbricks-test-files/assets/quotes.json
	inline = <<[
        <h1>Quotes</h1>
        <ul>
            {{range .Data.quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
    debug = false
}

````

**Expected Result**

````html
<h1>
  Quotes
</h1>
<ul>
  <li>
    <strong>
      Rumi:
    </strong>
    Your heart is the size of an ocean. Go find yourself in its hidden depths.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    The Bay of Bengal is hit frequently by cyclones. The months of November and May, in particular, are dangerous in this regard.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    Thinking is the capital, Enterprise is the way, Hard Work is the solution.
  </li>
  <li>
    <strong>
      Bill Gates:
    </strong>
    If You Can&#39;T Make It Good, At Least Make It Look Good.
  </li>
  <li>
    <strong>
      Rumi:
    </strong>
    Heart be brave. If you cannot be brave, just go. Love&#39;s glory is not a small thing.
  </li>
</ul>
````












## json enclose
#### enclose

**Description**  
The enclosing HTML element for the header divided by |


**Example**
````properties
local_json_test = <JSON_RENDER>
local_json_test {
	file =  hyperbricks-test-files/assets/quotes.json
	inline = <<[
        <h1>Quotes</h1>
        <ul>
            {{range .Data.quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
    debug = false
}

````

**Expected Result**

````html
<h1>
  Quotes
</h1>
<ul>
  <li>
    <strong>
      Rumi:
    </strong>
    Your heart is the size of an ocean. Go find yourself in its hidden depths.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    The Bay of Bengal is hit frequently by cyclones. The months of November and May, in particular, are dangerous in this regard.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    Thinking is the capital, Enterprise is the way, Hard Work is the solution.
  </li>
  <li>
    <strong>
      Bill Gates:
    </strong>
    If You Can&#39;T Make It Good, At Least Make It Look Good.
  </li>
  <li>
    <strong>
      Rumi:
    </strong>
    Heart be brave. If you cannot be brave, just go. Love&#39;s glory is not a small thing.
  </li>
</ul>
````












## json file
#### file

**Description**  
Path to the local JSON file


**Example**
````properties
local_json_test = <JSON_RENDER>
local_json_test {
	file =  hyperbricks-test-files/assets/quotes.json
	inline = <<[
        <h1>Quotes</h1>
        <ul>
            {{range .Data.quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
    debug = false
}

````

**Expected Result**

````html
<h1>
  Quotes
</h1>
<ul>
  <li>
    <strong>
      Rumi:
    </strong>
    Your heart is the size of an ocean. Go find yourself in its hidden depths.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    The Bay of Bengal is hit frequently by cyclones. The months of November and May, in particular, are dangerous in this regard.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    Thinking is the capital, Enterprise is the way, Hard Work is the solution.
  </li>
  <li>
    <strong>
      Bill Gates:
    </strong>
    If You Can&#39;T Make It Good, At Least Make It Look Good.
  </li>
  <li>
    <strong>
      Rumi:
    </strong>
    Heart be brave. If you cannot be brave, just go. Love&#39;s glory is not a small thing.
  </li>
</ul>
````












## json template
#### template

**Description**  
Template for rendering output


**Example**
````properties
local_json_test = <JSON_RENDER>
local_json_test {
	file =  hyperbricks-test-files/assets/quotes.json

    # this is a testfile with limitations, use {{TEMPLATE:sometemplate.html}} or use inline like here
	inline = <<[
        <h1>Quotes</h1>
        <ul>
            {{range .Data.quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
    debug = false
}

````

**Expected Result**

````html
<h1>
  Quotes
</h1>
<ul>
  <li>
    <strong>
      Rumi:
    </strong>
    Your heart is the size of an ocean. Go find yourself in its hidden depths.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    The Bay of Bengal is hit frequently by cyclones. The months of November and May, in particular, are dangerous in this regard.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    Thinking is the capital, Enterprise is the way, Hard Work is the solution.
  </li>
  <li>
    <strong>
      Bill Gates:
    </strong>
    If You Can&#39;T Make It Good, At Least Make It Look Good.
  </li>
  <li>
    <strong>
      Rumi:
    </strong>
    Heart be brave. If you cannot be brave, just go. Love&#39;s glory is not a small thing.
  </li>
</ul>
````












## json inline
#### inline

**Description**  
Use inline to define the template in a multiline block &lt;&lt;[ /* Template code goes here */ ]&gt;&gt;


**Example**
````properties
local_json_test = <JSON_RENDER>
local_json_test {
	file =  hyperbricks-test-files/assets/quotes.json
	inline = <<[
        <h1>Quotes</h1>
        <ul>
            {{range .Data.quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
    debug = false
}

````

**Expected Result**

````html
<h1>
  Quotes
</h1>
<ul>
  <li>
    <strong>
      Rumi:
    </strong>
    Your heart is the size of an ocean. Go find yourself in its hidden depths.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    The Bay of Bengal is hit frequently by cyclones. The months of November and May, in particular, are dangerous in this regard.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    Thinking is the capital, Enterprise is the way, Hard Work is the solution.
  </li>
  <li>
    <strong>
      Bill Gates:
    </strong>
    If You Can&#39;T Make It Good, At Least Make It Look Good.
  </li>
  <li>
    <strong>
      Rumi:
    </strong>
    Heart be brave. If you cannot be brave, just go. Love&#39;s glory is not a small thing.
  </li>
</ul>
````












## json values
#### values

**Description**  
Key-value pairs for template rendering


**Example**
````properties
local_json_test = <JSON_RENDER>
local_json_test {
	file =  hyperbricks-test-files/assets/quotes.json

    # this is a testfile with limitations, use {{TEMPLATE:sometemplate.html}} or use inline like here
	inline = <<[
        <h1>{{.someproperty}}</h1>
        <ul>
            {{range .Data.quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
    values {
        someproperty = Quotes!
    }
    debug = false
}

````

**Expected Result**

````html
<h1>
  Quotes!
</h1>
<ul>
  <li>
    <strong>
      Rumi:
    </strong>
    Your heart is the size of an ocean. Go find yourself in its hidden depths.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    The Bay of Bengal is hit frequently by cyclones. The months of November and May, in particular, are dangerous in this regard.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    Thinking is the capital, Enterprise is the way, Hard Work is the solution.
  </li>
  <li>
    <strong>
      Bill Gates:
    </strong>
    If You Can&#39;T Make It Good, At Least Make It Look Good.
  </li>
  <li>
    <strong>
      Rumi:
    </strong>
    Heart be brave. If you cannot be brave, just go. Love&#39;s glory is not a small thing.
  </li>
</ul>
````












## json debug
#### debug

**Description**  
Debug the response data


**Example**
````properties
local_json_test = <JSON_RENDER>
local_json_test {
	file =  hyperbricks-test-files/assets/quotes.json

    # this is a testfile with limitations, use {{TEMPLATE:sometemplate.html}} or use inline like here
	inline = <<[
        <h1>{{.someproperty}}</h1>
        <ul>
            {{range .Data.quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
    values {
        someproperty = Quotes!
    }
    debug = false
}

````

**Expected Result**

````html
<h1>
  Quotes!
</h1>
<ul>
  <li>
    <strong>
      Rumi:
    </strong>
    Your heart is the size of an ocean. Go find yourself in its hidden depths.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    The Bay of Bengal is hit frequently by cyclones. The months of November and May, in particular, are dangerous in this regard.
  </li>
  <li>
    <strong>
      Abdul Kalam:
    </strong>
    Thinking is the capital, Enterprise is the way, Hard Work is the solution.
  </li>
  <li>
    <strong>
      Bill Gates:
    </strong>
    If You Can&#39;T Make It Good, At Least Make It Look Good.
  </li>
  <li>
    <strong>
      Rumi:
    </strong>
    Heart be brave. If you cannot be brave, just go. Love&#39;s glory is not a small thing.
  </li>
</ul>
````












### Category: **menu**





<h3><a id="&lt;MENU&gt;">&lt;MENU&gt;</a></h3>

**Type Description**




**Properties**






### Category: **resources**





<h3><a id="&lt;CSS&gt;">&lt;CSS&gt;</a></h3>

**Type Description**






**Properties**

- [enclose](#css-enclose)





## css enclose
#### enclose

**Description**  
A custom &lt;style&gt; tag definition |. Will override extraAttributes.


**Example**
````properties
head = <HEAD>
head {
    10 = <CSS>
    10.file = hyperbricks-test-files/assets/styles.css
    10.attributes {
        media = screen
    }
    10.enclose = <style media="print">|</style>
}

````

**Expected Result**

````html
<head>
  <style media="print">
    body {
    background-color: red;
    }
  </style>
  <meta name="generator" content="hyperbricks cms">
</head>
````













<h3><a id="&lt;IMAGE&gt;">&lt;IMAGE&gt;</a></h3>

**Type Description**




























**Properties**

- [attributes](#image-attributes)
- [enclose](#image-enclose)
- [src](#image-src)
- [width](#image-width)
- [height](#image-height)
- [alt](#image-alt)
- [title](#image-title)
- [id](#image-id)
- [class](#image-class)
- [quality](#image-quality)
- [loading](#image-loading)
- [is_static](#image-is_static)





## image attributes
#### attributes

**Description**  
Extra attributes like loading, data-role, data-action etc


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 100
image.attributes {
  usemap = #catmap 
}

````

**Expected Result**

````html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" usemap="#catmap" />
````












## image enclose
#### enclose

**Description**  
Use the pipe symbol | to enclose the ````&lt;IMG&gt;```` tag.


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 100
image.attributes {
    loading = lazy
}
image.enclose = <div id="#gallery">|</div>

````

**Expected Result**

````html
<div id="#gallery">
  <img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" loading="lazy" />
</div>
````












## image src
#### src

**Description**  
The source URL of the image


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 300
image.height = 300
image.attributes {
    loading = lazy
}
image.enclose = <div id="#logo">|</div>

````

**Expected Result**

````html
<div id="#logo">
  <img src="static/images/cute_cat_w300_h300.jpg" width="300" height="300" loading="lazy" />
</div>
````












## image width
#### width

**Description**  
The width of the image (can be a number or percentage)


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 300
image.height = 300
image.attributes {
    loading = lazy
}
image.enclose = <div id="#logo">|</div>

````

**Expected Result**

````html
<div id="#logo">
  <img src="static/images/cute_cat_w300_h300.jpg" width="300" height="300" loading="lazy" />
</div>
````












## image height
#### height

**Description**  
The height of the image (can be a number or percentage)


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 310
image.height = 310
image.attributes {
    loading = lazy
}
image.enclose = <div id="#logo">|</div>

````

**Expected Result**

````html
<div id="#logo">
  <img src="static/images/cute_cat_w310_h310.jpg" width="310" height="310" loading="lazy" />
</div>
````












## image alt
#### alt

**Description**  
Alternative text for the image


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 100
image.alt = Cute cat!
image.enclose = <div id="#gallery">|</div>

````

**Expected Result**

````html
<div id="#gallery">
  <img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" alt="Cute cat!" />
</div>
````












## image title
#### title

**Description**  
The title attribute of the image


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 100
image.title = Some Cute Cat!

````

**Expected Result**

````html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" title="Some Cute Cat!" />
````












## image id
#### id

**Description**  
Id of image


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 310
image.height = 310
image.id = #cat

````

**Expected Result**

````html
<img src="static/images/cute_cat_w310_h310.jpg" width="310" height="310" id="#cat" />
````












## image class
#### class

**Description**  
CSS class for styling the image


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 100
image.title = Some Cute Cat!
image.class = aclass bclass cclass

````

**Expected Result**

````html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" title="Some Cute Cat!" class="aclass bclass cclass" />
````












## image quality
#### quality

**Description**  
Image quality for optimization, bigger is better.


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 320
image.height = 320
image.quality = 1

````

**Expected Result**

````html
<img src="static/images/cute_cat_w320_h320.jpg" width="320" height="320" />
````












## image loading
#### loading

**Description**  
Lazy loading strategy (e.g., &#39;lazy&#39;, &#39;eager&#39;)


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 320
image.height = 320
image.loading = lazy

````

**Expected Result**

````html
<img src="static/images/cute_cat_w320_h320.jpg" width="320" height="320" loading="lazy" />
````












## image is_static
#### is_static

**Description**  
Flag indicating if the image is static, if so the img will not be scaled and has to be present in the configured static image directory. See package.hyperbricks in the module for settings. 
```
#conveys this logic:
destDir := hbConfig.Directories[&#34;static&#34;] &#43; &#34;/images/&#34;
if config.IsStatic {
    destDir = hbConfig.Directories[&#34;render&#34;] &#43; &#34;/images/&#34;
}
```


**Example**
````properties
image = <IMAGE>
image.src = cute_cat.jpg
image.width = 310
image.height = 310
image.is_static = true

````

**Expected Result**

````html
<img src="static/images/cute_cat.jpg" />
````













<h3><a id="&lt;IMAGES&gt;">&lt;IMAGES&gt;</a></h3>

**Type Description**


























**Properties**

- [attributes](#images-attributes)
- [enclose](#images-enclose)
- [directory](#images-directory)
- [width](#images-width)
- [height](#images-height)
- [id](#images-id)
- [class](#images-class)
- [alt](#images-alt)
- [title](#images-title)
- [quality](#images-quality)
- [loading](#images-loading)





## images attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action


**Example**
````properties
images = <IMAGES>
images.directory = hyperbricks-test-files/assets/
images.width = 100
images.loading = lazy
images.id = #galleryimage_
images.attributes {
    decoding = async 
}

````

**Expected Result**

````html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" id="#galleryimage_0" loading="lazy" decoding="async" />
<img src="static/images/same_cute_cat_w100_h100.jpg" width="100" height="100" id="#galleryimage_1" loading="lazy" decoding="async" />
````












## images enclose
#### enclose

**Description**  
Use the pipe symbol | to enclose the ````&lt;IMG&gt;```` tag.


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 100
image.attributes {
    loading = lazy
}
image.enclose = <div id="#gallery">|</div>

````

**Expected Result**

````html
<div id="#gallery">
  <img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" loading="lazy" />
</div>
````












## images directory
#### directory

**Description**  
The directory path containing the images


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 100
image.attributes {
    loading = lazy
}
image.enclose = <div id="#gallery">|</div>

````

**Expected Result**

````html
<div id="#gallery">
  <img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" loading="lazy" />
</div>
````












## images width
#### width

**Description**  
The width of the images (can be a number or percentage)


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 330

````

**Expected Result**

````html
<img src="static/images/cute_cat_w330_h330.jpg" width="330" height="330" />
````












## images height
#### height

**Description**  
The height of the images (can be a number or percentage)


**Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.height = 100

````

**Expected Result**

````html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" />
````












## images id
#### id

**Description**  
Id of images with a index added to it


**Example**
````properties
images = <IMAGES>
images.directory = hyperbricks-test-files/assets/
images.width = 100
images.loading = lazy
images.id = #img_
images.attributes {
    decoding = async 
}

````

**Expected Result**

````html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" id="#img_0" loading="lazy" decoding="async" />
<img src="static/images/same_cute_cat_w100_h100.jpg" width="100" height="100" id="#img_1" loading="lazy" decoding="async" />
````












## images class
#### class

**Description**  
CSS class for styling the image


**Example**
````properties
images = <IMAGES>
images.directory = hyperbricks-test-files/assets/
images.width = 100
images.height = 10
images.loading = lazy
images.id = #galleryimage_
images.class = galleryimage bordered
images.attributes {
    decoding = async 
}

````

**Expected Result**

````html
<img src="static/images/cute_cat_w100_h10.jpg" width="100" height="10" class="galleryimage bordered" id="#galleryimage_0" loading="lazy" decoding="async" />
<img src="static/images/same_cute_cat_w100_h10.jpg" width="100" height="10" class="galleryimage bordered" id="#galleryimage_1" loading="lazy" decoding="async" />
````












## images alt
#### alt

**Description**  
Alternative text for the image


**Example**
````properties
images = <IMAGES>
images.directory = hyperbricks-test-files/assets/
images.width = 100
images.height = 10
images.loading = lazy
images.id = #galleryimage_
images.class = galleryimage bordered
images.alt = gallery image

````

**Expected Result**

````html
<img src="static/images/cute_cat_w100_h10.jpg" width="100" height="10" alt="gallery image" class="galleryimage bordered" id="#galleryimage_0" loading="lazy" />
<img src="static/images/same_cute_cat_w100_h10.jpg" width="100" height="10" alt="gallery image" class="galleryimage bordered" id="#galleryimage_1" loading="lazy" />
````












## images title
#### title

**Description**  
The title attribute of the image


**Example**
````properties
images = <IMAGES>
images.directory = hyperbricks-test-files/assets/
images.width = 100
images.loading = lazy
images.id = #img_
images.title = sometitle

````

**Expected Result**

````html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" title="sometitle" id="#img_0" loading="lazy" />
<img src="static/images/same_cute_cat_w100_h100.jpg" width="100" height="100" title="sometitle" id="#img_1" loading="lazy" />
````












## images quality
#### quality

**Description**  
Image quality for optimization


**Example**
````properties
images = <IMAGES>
images.directory = hyperbricks-test-files/assets/
images.width = 100
images.loading = lazy
images.id = #img_
images.quality = 1

````

**Expected Result**

````html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" id="#img_0" loading="lazy" />
<img src="static/images/same_cute_cat_w100_h100.jpg" width="100" height="100" id="#img_1" loading="lazy" />
````












## images loading
#### loading

**Description**  
Lazy loading strategy (e.g., &#39;lazy&#39;, &#39;eager&#39;)


**Example**
````properties
images = <IMAGES>
images.directory = hyperbricks-test-files/assets/
images.width = 100
images.loading = lazy
images.id = #img_
images.loading = lazy

````

**Expected Result**

````html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" id="#img_0" loading="lazy" />
<img src="static/images/same_cute_cat_w100_h100.jpg" width="100" height="100" id="#img_1" loading="lazy" />
````













<h3><a id="&lt;JS&gt;">&lt;JS&gt;</a></h3>

**Type Description**








**Properties**

- [attributes](#javascript-attributes)
- [enclose](#javascript-enclose)





## javascript attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action, type


**Example**
````properties
head = <HEAD>
head {
    10 = <JAVASCRIPT>
    10.file = hyperbricks-test-files/assets/main.js
    10.attributes {
        type = text/javascript
    }
}

````

**Expected Result**

````html
<head>
  <script type="text/javascript">
    console.log("Hello World!")
  </script>
  <meta name="generator" content="hyperbricks cms">
</head>
````












## javascript enclose
#### enclose

**Description**  
The enclosing HTML element for the header divided by |


**Example**
````properties
head = <HEAD>
head {
    10 = <JAVASCRIPT>
    10.file = hyperbricks-test-files/assets/main.js
    10.attributes {
        type = text/javascript
    }
    10.enclose = <script defer></script>
}

````

**Expected Result**

````html
<head>
<script defer></script>
console.log("Hello World!")
<meta name="generator" content="hyperbricks cms">
````











