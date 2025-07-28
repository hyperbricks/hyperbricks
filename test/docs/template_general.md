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