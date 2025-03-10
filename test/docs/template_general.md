### Defining Hypermedia Documents and Fragments

Hypermedia documents or fragments can be declared using simple key-value properties. This next example creates two locations ons site root (index) and /somefragment

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

