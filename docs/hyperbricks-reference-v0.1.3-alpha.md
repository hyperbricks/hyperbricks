

**Note:**  

This project and it's documentation is currently incomplete and in the experimental phase. It is recommended to use the code only for exploration or from a developer's perspective at this stage. If you're interested, you can explore the components and composite renderers in the `internal/component` and `internal/composite` directories.

The `<API>` type is currently undocumented because it is still evolving and likely to change. Note that `<API>` and `<API_RENDER>` are distinct types.". 

- **`<API_RENDER>`**: Renders a template using data fetched from an external endpoint.  
- **`<API>`**: Integrates with a `<MODEL>`, connects to a database, and simplifies common use cases (such as CRUD operations) with minimal configuration.

# HyperBricks
**Licence:** MIT  
**Version:** v0.1.3-alpha  
**Build time:** 2025-01-27T15:39:36Z   

Go direct to:

- [HyperBricks type reference](#hyperbricks-type-reference)
- [Installation](#installation)

- [Defining Hypermedia Documents and Fragments](#defining-hypermedia-documents-and-fragments)
- [Adding Properties to Configurations](#adding-properties-to-configurations)
- [Rendering Order and Property Rules](#rendering-order-and-property-rules)
- [Example Configurations](#example-configurations)
  - [Hypermedia Example](#hypermedia-example)
  - [Fragment Example with HTMX Trigger](#fragment-example-with-htmx-trigger)
- [Object Inheritance and Reusability](#object-inheritance-and-reusability)
- [Importing Predefined HyperScripts](#importing-predefined-hyperscripts)


## HyperBricks Documentation

HyperBricks aims to bridge front and back-end development of [htmx](https://htmx.org/) powered hypermedia applications using nested declarative configuration files. These configuration files (referred to as "hyperbricks") allow you to declare and describe the state of a document in a concise and structured manner.


### Defining Hypermedia Documents and Fragments

Hypermedia documents or fragments can be declared using simple key-value properties:

```properties
myHypermedia = <HYPERMEDIA>
# Or
myFragment = <FRAGMENT>
```

### Adding Properties to Configurations

You can add properties to hypermedia objects in either flat or nested formats:

**Flat Configuration Example:**
```properties
fragment = <FRAGMENT>
fragment.content = <TREE>
fragment.content.10 = <HTML>
fragment.content.10.value = <p>THIS IS HTML</p>
```

**Nested Configuration Example:**
```properties
fragment = <FRAGMENT>
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

### Rendering Order and Property Rules

Properties are rendered in alphanumeric order. They are typeless, meaning quotes are not required because at parsing hyperbricks types like ```<IMAGE>```, ```<HTML>``` or ```<TEXT>``` will be typed automaticly.

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


### Installation

To install HyperBricks, use the following command:

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
```

This command downloads and installs the HyperBricks CLI tool on your system.
---

### Initializing a Project

To initialize a new HyperBricks project, use the `init` command:

```bash
hyperbricks init -m <name-of-hyperbricks-module>
```

without the -m and ```<name-of-hyperbricks-module>``` this will create a ```default``` folder.


This will create a `package.hyperbricks` configuration file and set up the required directories for your project.

---

### Starting a Module

Once your project is initialized, start the HyperBricks server using the `start` command:

```bash
hyperbricks start  -m <name-of-hyperbricks-module>
```
This will launch the server, allowing you to manage and serve hypermedia content on the ip of your machine.

Or ```hyperbricks start``` for running the module named ```default```.

### Additional Commands

HyperBricks provides other useful commands:

- **`completion`**: Generate shell autocompletion scripts for supported shells.
- **`help`**: Display help information for any command.

For detailed usage information about a specific command, run:

```bash
hyperbricks [command] --help
```

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

- [&lt;API_RENDER&gt;](#<API_RENDER>) 
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

**&lt;HTML&gt; Type Description**










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

**&lt;TEXT&gt; Type Description**








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


<h3><a id="&lt;FRAGMENT&gt;">&lt;FRAGMENT&gt;</a></h3>

**&lt;FRAGMENT&gt; Type Description**












































**Properties**

- [response](#fragment-response)

- [title](#fragment-title)
- [route](#fragment-route)
- [section](#fragment-section)
- [enclose](#fragment-enclose)
- [template](#fragment-template)
- [static](#fragment-static)
- [index](#fragment-index)
- [hx_location](#fragment-hx_location)
- [hx_push_url](#fragment-hx_push_url)
- [hx_redirect](#fragment-hx_redirect)
- [hx_refresh](#fragment-hx_refresh)
- [hx_replace_url](#fragment-hx_replace_url)
- [hx_reswap](#fragment-hx_reswap)
- [hx_retarget](#fragment-hx_retarget)
- [hx_reselect](#fragment-hx_reselect)
- [hx_trigger](#fragment-hx_trigger)
- [hx_trigger_after_settle](#fragment-hx_trigger_after_settle)
- [hx_trigger_after_swap](#fragment-hx_trigger_after_swap)





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












## fragment template
#### template

**Description**  
Template configurations for rendering the fragment. (This will disable rendering any content added to the alpha numeric items that are added to the fragment root object.) See &lt;TEMPLATE&gt; for more details using templates.


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	template {
        template = <<[
            <div>{{content}}</div>

        ]>>
        isTemplate = true
        values {
            content = <HTML>
            content.value = <p>SOME HTML CONTENT</p>
        }
    }
}

````

**Expected Result**

````html
<div>
  <p>
    SOME HTML CONTENT
  </p>
</div>
````












## fragment static
#### static

**Description**  
Static file path associated with the fragment, this will only work for a hx-get (GET) request. 


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	static = some_static_file.extension
}

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










## fragment hx_location
#### hx_location

**Description**  
Allows you to do a client-side redirect that does not do a full page reload


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_location = someurl
    }
}

````










## fragment hx_push_url
#### hx_push_url

**Description**  
Pushes a new url into the history stack


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_push_url = /some/url
    }
}

````










## fragment hx_redirect
#### hx_redirect

**Description**  
Can be used to do a client-side redirect to a new location


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_redirect = /some/new/location
    }
}

````










## fragment hx_refresh
#### hx_refresh

**Description**  
If set to &#39;true&#39; the client-side will do a full refresh of the page


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_refresh = true
    }
}

````










## fragment hx_replace_url
#### hx_replace_url

**Description**  
replaces the current url in the location bar


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_replace_url = /alternative/url
    }
}

````










## fragment hx_reswap
#### hx_reswap

**Description**  
Allows you to specify how the response will be swapped. See hx-swap in the [HTMX documentation](https://htmx.org/).


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_reswap = innerHTML
    }
}

````










## fragment hx_retarget
#### hx_retarget

**Description**  
A css selector that updates the target of the content update


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_retarget = #someid
    }
}

````










## fragment hx_reselect
#### hx_reselect

**Description**  
A css selector that allows you to choose which part of the response is used to be swapped in.


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_reselect = #someotherid
    }
}

````










## fragment hx_trigger
#### hx_trigger

**Description**  
allows you to trigger client-side events


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_trigger = myEvent
    }
}

````










## fragment hx_trigger_after_settle
#### hx_trigger_after_settle

**Description**  
allows you to trigger client-side events after the settle step


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_trigger_after_settle = myAfterSettleEvent
    }
}

````










## fragment hx_trigger_after_swap
#### hx_trigger_after_swap

**Description**  
allows you to trigger client-side events after the swap step


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_trigger_after_swap = myAfterSwapEvent
    }
}

````








<h3><a id="&lt;HEAD&gt;">&lt;HEAD&gt;</a></h3>

**&lt;HEAD&gt; Type Description**














**Properties**

- [title](#head-title)
- [favicon](#head-favicon)
- [meta](#head-meta)
- [css](#head-css)
- [js](#head-js)





## head title
#### title

**Description**  
The title of the hypermedia document


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.head = <HEAD>
hypermedia.head {
    title = Home
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <title>
      Home
    </title>
  </head>
  <body></body>
</html>
````












## head favicon
#### favicon

**Description**  
Path to the favicon for the hypermedia document


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.head = <HEAD>
hypermedia.head {
    favicon = /images/icon.ico
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <link rel="icon" type="image/x-icon" href="/images/icon.ico">
  </head>
  <body></body>
</html>
````












## head meta
#### meta

**Description**  
Metadata for the head section


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.head = <HEAD>
hypermedia.head {
    meta {
        a = b
        b = c
    }
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <meta name="a" content="b">
    <meta name="b" content="c">
  </head>
  <body></body>
</html>
````












## head css
#### css

**Description**  
CSS files to include


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.head = <HEAD>
hypermedia.head {
    css = [style.css,morestyles.css]
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <link rel="stylesheet" href="style.css">
    <link rel="stylesheet" href="morestyles.css">
  </head>
  <body></body>
</html>
````












## head js
#### js

**Description**  
JavaScript files to include


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.head = <HEAD>
hypermedia.head {
    js = [main.js,helpers.js]
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <script src="main.js"></script>
    <script src="helpers.js"></script>
  </head>
  <body></body>
</html>
````










<h3><a id="&lt;HYPERMEDIA&gt;">&lt;HYPERMEDIA&gt;</a></h3>

**&lt;HYPERMEDIA&gt; Type Description**






























**Properties**


- [title](#hypermedia-title)
- [route](#hypermedia-route)
- [section](#hypermedia-section)
- [bodytag](#hypermedia-bodytag)
- [enclose](#hypermedia-enclose)
- [favicon](#hypermedia-favicon)
- [template](#hypermedia-template)
- [static](#hypermedia-static)
- [index](#hypermedia-index)
- [doctype](#hypermedia-doctype)
- [htmltag](#hypermedia-htmltag)
- [head](#hypermedia-head)








## hypermedia title
#### title

**Description**  
The title of the hypermedia site


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia {
    title = Home
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <title>
      Home
    </title>
  </head>
  <body></body>
</html>
````












## hypermedia route
#### route

**Description**  
The route (URL-friendly identifier) for the hypermedia


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia {
    route = index
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <body></body>
</html>
````












## hypermedia section
#### section

**Description**  
The section the hypermedia belongs to. This can be used with the component &lt;MENU&gt; for example.


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia {
    section = my_section
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <body></body>
</html>
````












## hypermedia bodytag
#### bodytag

**Description**  
Special body enclosure with use of a pipe symbol |. Please note that this will not work when a template is applied. In that case, you have to add the bodytag in the template.


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = index
hypermedia.bodytag = <body id="main">|</body>
hypermedia.10 = <TEXT>
hypermedia.10.value = HELLO WORLD!

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <body id="main">
    HELLO WORLD!
  </body>
</html>
````













## hypermedia enclose
#### enclose

**Description**  
Enclosure of the property for the hypermedia


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = index
hypermedia.bodytag = <body id="main">|</body>
hypermedia.10 = <TEXT>
hypermedia.10.value = HELLO WORLD!
hypermedia.enclose = <p>|</p>

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <body id="main">
    <p>
      HELLO WORLD!
    </p>
  </body>
</html>
````













## hypermedia favicon
#### favicon

**Description**  
Path to the favicon for the hypermedia


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia {
    favicon = static/favicon.ico
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <link rel="icon" type="image/x-icon" href="static/favicon.ico">
  </head>
  <body></body>
</html>
````













## hypermedia template
#### template

**Description**  
Template configurations for rendering the hypermedia. See &lt;TEMPLATE&gt; for field descriptions.


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia {
	template {
        template = <<[
            <div>{{content}}</div>

        ]>>
        isTemplate = true
        values {
            content = <HTML>
            content.value = <p>SOME HTML CONTENT</p>
        }
    }
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <body>
    <div>
      <p>
        SOME HTML CONTENT
      </p>
    </div>
  </body>
</html>
````












## hypermedia static
#### static

**Description**  
Static file path associated with the hypermedia, for rendering out the hypermedia to static files.


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia {
	static = index.html
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <body></body>
</html>
````












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












## hypermedia doctype
#### doctype

**Description**  
Alternative Doctype for the HTML document


**Example**
````properties
hypermedia = <HYPERMEDIA>
# this is just an example of an alternative doctype configuration
hypermedia.doctype = <!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">

````

**Expected Result**

````html
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html>
  <body></body>
</html>
````












## hypermedia htmltag
#### htmltag

**Description**  
The opening HTML tag with attributes


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.htmltag = <html lang="en">

````

**Expected Result**

````html
<!DOCTYPE html>
<html lang="en">
  <body></body>
</html>
````












## hypermedia head
#### head

**Description**  
Builds header content. See &lt;HEADER&gt; for details


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = index
hypermedia.head = <HEAD>
hypermedia.head {
    css = [styles.css,xxxx]
    js = [styles.css,xxxx]

    meta {
        a = b
        b = c
    }
    999 = <HTML>
    999.value = <!-- 999 overides default generator meta tag -->

    1001 = <CSS>
    1001.inline = <<[
        body {
            pading:10px;
        }
    ]>>

    20 = <HTML>
    20.value = <meta name="generator" content="hyperbricks cms">
     
}
hypermedia.10 = <HTML>
hypermedia.10.value = <p>some HTML</p>

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <!-- 999 overides default generator meta tag -->
    <meta name="a" content="b">
    <meta name="b" content="c">
    <link rel="stylesheet" href="styles.css">
    <link rel="stylesheet" href="xxxx">
    <script src="styles.css"></script>
    <script src="xxxx"></script>
    <style>
      body {
      pading:10px;
      }
    </style>
  </head>
  <body>
    <p>
      some HTML
    </p>
  </body>
</html>
````










<h3><a id="&lt;TEMPLATE&gt;">&lt;TEMPLATE&gt;</a></h3>

**&lt;TEMPLATE&gt; Type Description**














**Properties**


- [template](#template-template)
- [istemplate](#template-istemplate)
- [values](#template-values)
- [enclose](#template-enclose)








## template template
#### template

**Description**  
The template used for rendering.


**Example**
````properties
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

    20 < myComponent

    enclose = <div class="youtube_video">|</div>
}

````

**Expected Result**

````html
<div class="youtube_video">
  <iframe width="300" height="400" src="https://www.youtube.com/watch?v=Wlh6yFSJEms"></iframe>
  <iframe width="300" height="400" src="https://www.youtube.com/embed/tgbNymZ7vqY"></iframe>
</div>
````












## template istemplate
#### istemplate

**Description**  
Determines if the field is a inline template or when not defined a reference to a template file


**Example**
````properties
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
myComponent.enclose = |

fragment = <FRAGMENT>
fragment.content = <TREE>
fragment.content {
    10 < myComponent
    10.values.src = https://www.youtube.com/watch?v=Wlh6yFSJEms

    20 < myComponent

    enclose = <div class="youtube_video">|</div>
}

````

**Expected Result**

````html
<div class="youtube_video">
  <iframe width="300" height="400" src="https://www.youtube.com/watch?v=Wlh6yFSJEms"></iframe>
  <iframe width="300" height="400" src="https://www.youtube.com/embed/tgbNymZ7vqY"></iframe>
</div>
````












## template values
#### values

**Description**  
Key-value pairs for template rendering


**Example**
````properties

$test = hello world

myComponent = <TEMPLATE>
myComponent {
    template = <<[
        <h1>{{header}}</h1>
        <p>{{text}}</p>
    ]>>
    istemplate = true
    values {
        header = {{VAR:test}}!
        text = some text
    }
}

fragment = <FRAGMENT>
fragment.content = <TREE>
fragment.content {
    10 < myComponent
    enclose = <div class="sometext">|</div>
}

````

**Expected Result**

````html
<div class="sometext">
  <h1>
    hello world!
  </h1>
  <p>
    some text
  </p>
</div>
````












## template enclose
#### enclose

**Description**  
Enclosing property for the template rendered output divided by |


**Example**
````properties
myComponent = <TEMPLATE>
myComponent {
    template = <<[
      <img src="{{src}}" alt="{{alt}}" width="{{width}}" height="{{height}}">
    ]>>
    istemplate = true
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

**&lt;TREE&gt; Type Description**








**Properties**


- [enclose](#tree-enclose)








## tree enclose
#### enclose

**Description**  
Enclosing tag using the pipe symbol |


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	10 = <TREE>
    10 {
        10 = <TREE>
        10 {
            1 = <HTML>
            1.value = <p>SOME NESTED HTML --- 10-1</p>

            2 = <HTML>
            2.value = <p>SOME NESTED HTML --- 10-2</p>
        }

        20 = <TREE>
        20 {
            1 = <HTML>
            1.value = <p>SOME NESTED HTML --- 20-1</p>
            
            2 = <HTML>
            2.value = <p>SOME NESTED HTML --- 20-2</p>
        }
        enclose = <div>|</div>
    }
}

````

**Expected Result**

````html
<div>
  <p>
    SOME NESTED HTML --- 10-1
  </p>
  <p>
    SOME NESTED HTML --- 10-2
  </p>
  <p>
    SOME NESTED HTML --- 20-1
  </p>
  <p>
    SOME NESTED HTML --- 20-2
  </p>
</div>
````












### Category: **data**


<h3><a id="&lt;API_RENDER&gt;">&lt;API_RENDER&gt;</a></h3>

**&lt;API_RENDER&gt; Type Description**
























**Properties**

- [enclose](#api_render-enclose)
- [endpoint](#api_render-endpoint)
- [method](#api_render-method)
- [headers](#api_render-headers)
- [body](#api_render-body)
- [template](#api_render-template)
- [istemplate](#api_render-istemplate)
- [user](#api_render-user)
- [pass](#api_render-pass)
- [debug](#api_render-debug)





## api_render enclose
#### enclose

**Description**  
The enclosing HTML element for the header divided by |


**Example**
````properties
# use user and pass for cases with basic authentication
api_test = <API_RENDER>
api_test {
	endpoint = https://dummyjson.com/auth/login
	method = POST
	body = {"username":"emilys","password":"emilyspass","expiresInMins":30}
	user = emilys
	pass = emilyspass
	headers {
		Access-Control-Allow-Credentials = true
		Content-Type = application/json
	}
	template = <<[
		<ul id="{{index .id}}">
			<li>{{index .firstName}} {{index .lastName}}</li>
		<ul>
	]>>
	istemplate = true
	debug = false
	enclose = <div class="userlist">|</div>
}

````

**Expected Result**

````html
<div class="userlist">
  <ul id="1">
  <li>
    Emily Johnson
  </li>
  <ul>
</div>
````












## api_render endpoint
#### endpoint

**Description**  
The API endpoint


**Example**
````properties
# use user and pass for cases with basic authentication
api_test = <API_RENDER>
api_test {
	endpoint = https://dummyjson.com/auth/login
	method = POST
	body = {"username":"emilys","password":"emilyspass","expiresInMins":30}
	user = emilys
	pass = emilyspass
	headers {
		Access-Control-Allow-Credentials = true
		Content-Type = application/json
	}
	template = <<[
		<ul id="{{index .id}}">
			<li>{{index .firstName}} {{index .lastName}}</li>
		<ul>
	]>>
	istemplate = true
	debug = false
	enclose = <div class="userlist">|</div>
}

````

**Expected Result**

````html
<div class="userlist">
  <ul id="1">
  <li>
    Emily Johnson
  </li>
  <ul>
</div>
````












## api_render method
#### method

**Description**  
HTTP method to use for API calls, GET POST PUT DELETE etc... 


**Example**
````properties
# use user and pass for cases with basic authentication
api_test = <API_RENDER>
api_test {
	endpoint = https://dummyjson.com/auth/login
	method = POST
	body = {"username":"emilys","password":"emilyspass","expiresInMins":30}
	user = emilys
	pass = emilyspass
	headers {
		Access-Control-Allow-Credentials = true
		Content-Type = application/json
	}
	template = <<[
		<ul id="{{index .id}}">
			<li>{{index .firstName}} {{index .lastName}}</li>
		<ul>
	]>>
	istemplate = true
	debug = false
	enclose = <div class="userlist">|</div>
}

````

**Expected Result**

````html
<div class="userlist">
  <ul id="1">
  <li>
    Emily Johnson
  </li>
  <ul>
</div>
````












## api_render headers
#### headers

**Description**  
Optional HTTP headers for API requests


**Example**
````properties
# use user and pass for cases with basic authentication
api_test = <API_RENDER>
api_test {
	endpoint = https://dummyjson.com/auth/login
	method = POST
	body = {"username":"emilys","password":"emilyspass","expiresInMins":30}
	user = emilys
	pass = emilyspass
	headers {
		Access-Control-Allow-Credentials = true
		Content-Type = application/json
	}
	template = <<[
		<ul id="{{index .id}}">
			<li>{{index .firstName}} {{index .lastName}}</li>
		<ul>
	]>>
	istemplate = true
	debug = false
	enclose = <div class="userlist">|</div>
}

````

**Expected Result**

````html
<div class="userlist">
  <ul id="1">
  <li>
    Emily Johnson
  </li>
  <ul>
</div>
````












## api_render body
#### body

**Description**  
Use the string format of the example, do not use an nested object to define. The values will be parsed en send with the request.


**Example**
````properties
# use user and pass for cases with basic authentication
api_test = <API_RENDER>
api_test {
	endpoint = https://dummyjson.com/auth/login
	method = POST
	body = {"username":"emilys","password":"emilyspass","expiresInMins":30}
	user = emilys
	pass = emilyspass
	headers {
		Access-Control-Allow-Credentials = true
		Content-Type = application/json
	}
	template = <<[
		<ul id="{{index .id}}">
			<li>{{index .firstName}} {{index .lastName}}</li>
		<ul>
	]>>
	istemplate = true
	debug = false
	enclose = <div class="userlist">|</div>
}

````

**Expected Result**

````html
<div class="userlist">
  <ul id="1">
  <li>
    Emily Johnson
  </li>
  <ul>
</div>
````












## api_render template
#### template

**Description**  
Template used for rendering API output


**Example**
````properties
# use user and pass for cases with basic authentication
api_test = <API_RENDER>
api_test {
	endpoint = https://dummyjson.com/auth/login
	method = POST
	body = {"username":"emilys","password":"emilyspass","expiresInMins":30}
	user = emilys
	pass = emilyspass
	headers {
		Access-Control-Allow-Credentials = true
		Content-Type = application/json
	}
	template = <<[
		<ul id="{{index .id}}">
			<li>{{index .firstName}} {{index .lastName}}</li>
		<ul>
	]>>
	istemplate = true
	debug = false
	enclose = <div class="userlist">|</div>
}

````

**Expected Result**

````html
<div class="userlist">
  <ul id="1">
  <li>
    Emily Johnson
  </li>
  <ul>
</div>
````












## api_render istemplate
#### istemplate

**Description**  



**Example**
````properties
# use user and pass for cases with basic authentication
api_test = <API_RENDER>
api_test {
	endpoint = https://dummyjson.com/auth/login
	method = POST
	body = {"username":"emilys","password":"emilyspass","expiresInMins":30}
	user = emilys
	pass = emilyspass
	headers {
		Access-Control-Allow-Credentials = true
		Content-Type = application/json
	}
	template = <<[
		<ul id="{{index .id}}">
			<li>{{index .firstName}} {{index .lastName}}</li>
		<ul>
	]>>
	istemplate = true
	debug = false
	enclose = <div class="userlist">|</div>
}

````

**Expected Result**

````html
<div class="userlist">
  <ul id="1">
  <li>
    Emily Johnson
  </li>
  <ul>
</div>
````












## api_render user
#### user

**Description**  
User for basic auth


**Example**
````properties
# use user and pass for cases with basic authentication
api_test = <API_RENDER>
api_test {
	endpoint = https://dummyjson.com/auth/login
	method = POST
	body = {"username":"emilys","password":"emilyspass","expiresInMins":30}
	user = emilys
	pass = emilyspass
	headers {
		Access-Control-Allow-Credentials = true
		Content-Type = application/json
	}
	template = <<[
		<ul id="{{index .id}}">
			<li>{{index .firstName}} {{index .lastName}}</li>
		<ul>
	]>>
	istemplate = true
	debug = false
	enclose = <div class="userlist">|</div>
}

````

**Expected Result**

````html
<div class="userlist">
  <ul id="1">
  <li>
    Emily Johnson
  </li>
  <ul>
</div>
````












## api_render pass
#### pass

**Description**  
User for basic auth


**Example**
````properties
# use user and pass for cases with basic authentication
api_test = <API_RENDER>
api_test {
	endpoint = https://dummyjson.com/auth/login
	method = POST
	body = {"username":"emilys","password":"emilyspass","expiresInMins":30}
	user = emilys
	pass = emilyspass
	headers {
		Access-Control-Allow-Credentials = true
		Content-Type = application/json
	}
	template = <<[
		<ul id="{{index .id}}">
			<li>{{index .firstName}} {{index .lastName}}</li>
		<ul>
	]>>
	istemplate = true
	debug = false
	enclose = <div class="userlist">|</div>
}

````

**Expected Result**

````html
<div class="userlist">
  <ul id="1">
  <li>
    Emily Johnson
  </li>
  <ul>
</div>
````












## api_render debug
#### debug

**Description**  
Debug the response data


**Example**
````properties
# use user and pass for cases with basic authentication
api_test = <API_RENDER>
api_test {
	endpoint = https://dummyjson.com/auth/login
	method = POST
	body = {"username":"emilys","password":"emilyspass","expiresInMins":30}
	user = emilys
	pass = emilyspass
	headers {
		Access-Control-Allow-Credentials = true
		Content-Type = application/json
	}
	template = <<[
		<ul id="{{index .id}}">
			<li>{{index .firstName}} {{index .lastName}}</li>
		<ul>
	]>>
	istemplate = true
	debug = false
	enclose = <div class="userlist">|</div>
}

````

**Expected Result**

````html
<div class="userlist">
  <ul id="1">
  <li>
    Emily Johnson
  </li>
  <ul>
</div>
````










<h3><a id="&lt;JSON&gt;">&lt;JSON&gt;</a></h3>

**&lt;JSON&gt; Type Description**
















**Properties**

- [attributes](#json-attributes)
- [enclose](#json-enclose)
- [file](#json-file)
- [template](#json-template)
- [istemplate](#json-istemplate)
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
	template = <<[
        <h1>Quotes</h1>
        <ul>
            {{range .quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
	istemplate = true
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
	template = <<[
        <h1>Quotes</h1>
        <ul>
            {{range .quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
	istemplate = true
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
	template = <<[
        <h1>Quotes</h1>
        <ul>
            {{range .quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
	istemplate = true
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
	template = <<[
        <h1>Quotes</h1>
        <ul>
            {{range .quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
	istemplate = true
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












## json istemplate
#### istemplate

**Description**  



**Example**
````properties
local_json_test = <JSON_RENDER>
local_json_test {
	file =  hyperbricks-test-files/assets/quotes.json
	template = <<[
        <h1>Quotes</h1>
        <ul>
            {{range .quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
	istemplate = true
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












## json debug
#### debug

**Description**  
Debug the response data


**Example**
````properties
local_json_test = <JSON_RENDER>
local_json_test {
	file =  hyperbricks-test-files/assets/quotes.json
	template = <<[
        <h1>Quotes</h1>
        <ul>
            {{range .quotes}}
                <li><strong>{{.author}}:</strong> {{.quote}}</li>
            {{end}}
        </ul>
	]>>
	istemplate = true
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












### Category: **menu**


<h3><a id="&lt;MENU&gt;">&lt;MENU&gt;</a></h3>

**&lt;MENU&gt; Type Description**


















**Properties**

- [enclose](#menu-enclose)
- [section](#menu-section)
- [order](#menu-order)
- [sort](#menu-sort)
- [active](#menu-active)
- [item](#menu-item)
- [enclose](#menu-enclose)





## menu enclose
#### enclose

**Description**  
The enclosing HTML element for the header divided by |


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = doc
hypermedia.title = DOCUMENT
hypermedia.section = demo_main_menu
hypermedia.10 = <MENU>
hypermedia.10 {
    section = demo_main_menu
    sort = index
    order = asc
    active = <a class="nav-link fw-bold py-1 px-0 active" aria-current="page" href="#">{{ .Title }}</a>
    item = <a class="nav-link fw-bold py-1 px-0" href="{{ .Route }}"> {{ .Title }}</a>
    enclose = <nav class="nav nav-masthead justify-content-center float-md-end">|</nav>
}

hm_1 < hypermedia
hm_1.route = doc1
hm_1.title = DOCUMENT_1

hm_2 < hypermedia
hm_2.route = doc2
hm_2.title = DOCUMENT_2

hm_3 < hypermedia
hm_3.route = doc3
hm_3.title = DOCUMENT_3

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <title>
      DOCUMENT_3
    </title>
  </head>
  <body>
    <nav class="nav nav-masthead justify-content-center float-md-end">
      <a class="nav-link fw-bold py-1 px-0" href="doc1">
        DOCUMENT_1
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc2">
        DOCUMENT_2
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc3">
        DOCUMENT_3
      </a>
    </nav>
  </body>
</html>
````












## menu section
#### section

**Description**  
The section of the menu to display.


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = doc
hypermedia.title = DOCUMENT
hypermedia.section = demo_main_menu
hypermedia.10 = <MENU>
hypermedia.10 {
    section = demo_main_menu
    sort = index
    order = asc
    active = <a class="nav-link fw-bold py-1 px-0 active" aria-current="page" href="#">{{ .Title }}</a>
    item = <a class="nav-link fw-bold py-1 px-0" href="{{ .Route }}"> {{ .Title }}</a>
    enclose = <nav class="nav nav-masthead justify-content-center float-md-end">|</nav>
}

hm_1 < hypermedia
hm_1.route = doc1
hm_1.title = DOCUMENT_1

hm_2 < hypermedia
hm_2.route = doc2
hm_2.title = DOCUMENT_2

hm_3 < hypermedia
hm_3.route = doc3
hm_3.title = DOCUMENT_3

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <title>
      DOCUMENT_3
    </title>
  </head>
  <body>
    <nav class="nav nav-masthead justify-content-center float-md-end">
      <a class="nav-link fw-bold py-1 px-0" href="doc1">
        DOCUMENT_1
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc2">
        DOCUMENT_2
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc3">
        DOCUMENT_3
      </a>
    </nav>
  </body>
</html>
````












## menu order
#### order

**Description**  
The order of items in the menu (&#39;asc&#39; or &#39;desc&#39;).


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = doc
hypermedia.title = DOCUMENT
hypermedia.section = demo_main_menu
hypermedia.10 = <MENU>
hypermedia.10 {
    section = demo_main_menu
    sort = index
    order = asc
    active = <a class="nav-link fw-bold py-1 px-0 active" aria-current="page" href="#">{{ .Title }}</a>
    item = <a class="nav-link fw-bold py-1 px-0" href="{{ .Route }}"> {{ .Title }}</a>
    enclose = <nav class="nav nav-masthead justify-content-center float-md-end">|</nav>
}

hm_1 < hypermedia
hm_1.route = doc1
hm_1.title = DOCUMENT_1

hm_2 < hypermedia
hm_2.route = doc2
hm_2.title = DOCUMENT_2

hm_3 < hypermedia
hm_3.route = doc3
hm_3.title = DOCUMENT_3

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <title>
      DOCUMENT_3
    </title>
  </head>
  <body>
    <nav class="nav nav-masthead justify-content-center float-md-end">
      <a class="nav-link fw-bold py-1 px-0" href="doc1">
        DOCUMENT_1
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc2">
        DOCUMENT_2
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc3">
        DOCUMENT_3
      </a>
    </nav>
  </body>
</html>
````












## menu sort
#### sort

**Description**  
The field to sort menu items by (&#39;title&#39;, &#39;route&#39;, or &#39;index&#39;).


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = doc
hypermedia.title = DOCUMENT
hypermedia.section = demo_main_menu
hypermedia.10 = <MENU>
hypermedia.10 {
    section = demo_main_menu
    sort = index
    order = asc
    active = <a class="nav-link fw-bold py-1 px-0 active" aria-current="page" href="#">{{ .Title }}</a>
    item = <a class="nav-link fw-bold py-1 px-0" href="{{ .Route }}"> {{ .Title }}</a>
    enclose = <nav class="nav nav-masthead justify-content-center float-md-end">|</nav>
}

hm_1 < hypermedia
hm_1.route = doc1
hm_1.title = DOCUMENT_1

hm_2 < hypermedia
hm_2.route = doc2
hm_2.title = DOCUMENT_2

hm_3 < hypermedia
hm_3.route = doc3
hm_3.title = DOCUMENT_3

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <title>
      DOCUMENT_3
    </title>
  </head>
  <body>
    <nav class="nav nav-masthead justify-content-center float-md-end">
      <a class="nav-link fw-bold py-1 px-0" href="doc1">
        DOCUMENT_1
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc2">
        DOCUMENT_2
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc3">
        DOCUMENT_3
      </a>
    </nav>
  </body>
</html>
````












## menu active
#### active

**Description**  
Template for the active menu item.


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = doc
hypermedia.title = DOCUMENT
hypermedia.section = demo_main_menu
hypermedia.10 = <MENU>
hypermedia.10 {
    section = demo_main_menu
    sort = index
    order = asc
    active = <a class="nav-link fw-bold py-1 px-0 active" aria-current="page" href="#">{{ .Title }}</a>
    item = <a class="nav-link fw-bold py-1 px-0" href="{{ .Route }}"> {{ .Title }}</a>
    enclose = <nav class="nav nav-masthead justify-content-center float-md-end">|</nav>
}

hm_1 < hypermedia
hm_1.route = doc1
hm_1.title = DOCUMENT_1

hm_2 < hypermedia
hm_2.route = doc2
hm_2.title = DOCUMENT_2

hm_3 < hypermedia
hm_3.route = doc3
hm_3.title = DOCUMENT_3


````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <title>
      DOCUMENT_3
    </title>
  </head>
  <body>
    <nav class="nav nav-masthead justify-content-center float-md-end">
      <a class="nav-link fw-bold py-1 px-0" href="doc1">
        DOCUMENT_1
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc2">
        DOCUMENT_2
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc3">
        DOCUMENT_3
      </a>
    </nav>
  </body>
</html>
````












## menu item
#### item

**Description**  
Template for regular menu items.


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = doc
hypermedia.title = DOCUMENT
hypermedia.section = demo_main_menu
hypermedia.10 = <MENU>
hypermedia.10 {
    section = demo_main_menu
    sort = index
    order = asc
    active = <a class="nav-link fw-bold py-1 px-0 active" aria-current="page" href="#">{{ .Title }}</a>
    item = <a class="nav-link fw-bold py-1 px-0" href="{{ .Route }}"> {{ .Title }}</a>
    enclose = <nav class="nav nav-masthead justify-content-center float-md-end">|</nav>
}

hm_1 < hypermedia
hm_1.route = doc1
hm_1.title = DOCUMENT_1

hm_2 < hypermedia
hm_2.route = doc2
hm_2.title = DOCUMENT_2

hm_3 < hypermedia
hm_3.route = doc3
hm_3.title = DOCUMENT_3

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <title>
      DOCUMENT_3
    </title>
  </head>
  <body>
    <nav class="nav nav-masthead justify-content-center float-md-end">
      <a class="nav-link fw-bold py-1 px-0" href="doc1">
        DOCUMENT_1
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc2">
        DOCUMENT_2
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc3">
        DOCUMENT_3
      </a>
    </nav>
  </body>
</html>
````












## menu enclose
#### enclose

**Description**  
The enclosing HTML element for the header divided by |


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = doc
hypermedia.title = DOCUMENT
hypermedia.section = demo_main_menu
hypermedia.10 = <MENU>
hypermedia.10 {
    section = demo_main_menu
    sort = index
    order = asc
    active = <a class="nav-link fw-bold py-1 px-0 active" aria-current="page" href="#">{{ .Title }}</a>
    item = <a class="nav-link fw-bold py-1 px-0" href="{{ .Route }}"> {{ .Title }}</a>
    enclose = <nav class="nav nav-masthead justify-content-center float-md-end">|</nav>
}

hm_1 < hypermedia
hm_1.route = doc1
hm_1.title = DOCUMENT_1

hm_2 < hypermedia
hm_2.route = doc2
hm_2.title = DOCUMENT_2

hm_3 < hypermedia
hm_3.route = doc3
hm_3.title = DOCUMENT_3

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbricks cms">
    <title>
      DOCUMENT_3
    </title>
  </head>
  <body>
    <nav class="nav nav-masthead justify-content-center float-md-end">
      <a class="nav-link fw-bold py-1 px-0" href="doc1">
        DOCUMENT_1
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc2">
        DOCUMENT_2
      </a>
      <a class="nav-link fw-bold py-1 px-0" href="doc3">
        DOCUMENT_3
      </a>
    </nav>
  </body>
</html>
````












### Category: **resources**


<h3><a id="&lt;CSS&gt;">&lt;CSS&gt;</a></h3>

**&lt;CSS&gt; Type Description**














**Properties**

- [attributes](#css-attributes)
- [enclose](#css-enclose)
- [inline](#css-inline)
- [link](#css-link)
- [file](#css-file)





## css attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action, media


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.head {
    10 = <CSS>
    10.file = hyperbricks-test-files/assets/styles.css
    10.attributes {
        media = screen
    }
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <style media="screen">
      body {
      background-color: red;
      }
    </style>
    <meta name="generator" content="hyperbricks cms">
  </head>
  <body></body>
</html>
````












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












## css inline
#### inline

**Description**  
Use inline to define css in a multiline block &lt;&lt;[ /* css goes here */ ]&gt;&gt;


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.head {
    10 = <CSS>
    10.inline = <<[
        body {
            background-color: lightblue;
        }
    ]>>
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <style>
      body {
      background-color: lightblue;
      }
    </style>
    <meta name="generator" content="hyperbricks cms">
  </head>
  <body></body>
</html>
````












## css link
#### link

**Description**  
Use link for a link tag to a css file.


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.head = <HEAD>
hypermedia.head {
    10 = <CSS>
    10.link = styles.css
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <link rel="stylesheet" href="styles.css">
    <meta name="generator" content="hyperbricks cms">
  </head>
  <body></body>
</html>
````












## css file
#### file

**Description**  
file overrides link and inline, it loads contents of a file and renders it in a style tag.


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.head {
    10 = <CSS>
    10.file = hyperbricks-test-files/assets/styles.css
    10.attributes {
        media = screen
    }
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <style media="screen">
      body {
      background-color: red;
      }
    </style>
    <meta name="generator" content="hyperbricks cms">
  </head>
  <body></body>
</html>
````










<h3><a id="&lt;IMAGE&gt;">&lt;IMAGE&gt;</a></h3>

**&lt;IMAGE&gt; Type Description**




























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

**&lt;IMAGES&gt; Type Description**


























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

**&lt;JS&gt; Type Description**














**Properties**

- [attributes](#javascript-attributes)
- [enclose](#javascript-enclose)
- [inline](#javascript-inline)
- [link](#javascript-link)
- [file](#javascript-file)





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












## javascript inline
#### inline

**Description**  
Use inline to define JavaScript in a multiline block &lt;&lt;[ /* JavaScript goes here */ ]&gt;&gt;


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.head {
    10 = <JAVASCRIPT>
    10.inline = console.log("Hello World!")
    10.attributes {
        type = text/javascript
    }
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <script type="text/javascript">
      console.log("Hello World!")
    </script>
    <meta name="generator" content="hyperbricks cms">
  </head>
  <body></body>
</html>
````












## javascript link
#### link

**Description**  
Use link for a script tag with a src attribute


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.head {
    10 = <JAVASCRIPT>
    10.link = hyperbricks-test-files/assets/main.js
    10.attributes {
        type = text/javascript
    }
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <script src="hyperbricks-test-files/assets/main.js"></script>
    <meta name="generator" content="hyperbricks cms">
  </head>
  <body></body>
</html>
````












## javascript file
#### file

**Description**  
File overrides link and inline, it loads contents of a file and renders it in a script tag.


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.head {
    10 = <JAVASCRIPT>
    10.file = hyperbricks-test-files/assets/main.js
    10.attributes {
        type = text/javascript
    }
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <script type="text/javascript">
      console.log("Hello World!")
    </script>
    <meta name="generator" content="hyperbricks cms">
  </head>
  <body></body>
</html>
````











