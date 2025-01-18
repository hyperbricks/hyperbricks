## HyperBricks Documentation

HyperBricks is a powerful CMS that use nested declarative configuration files, enabling the rapid development of [htmx](https://htmx.org/) hypermedia-based applications.

This declarative configuration files (hyperbricks) allows to declare and describe state of a document.

Hypermedia documents or fragments are declared with:
&lt;HYPERMEDIA&gt; or &lt;FRAGMENT&gt;

### basic example ###
```properties
myComponent = <TEMPLATE>
myComponent.template {
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
    10.template.values.src = https://www.youtube.com/watch?v=Wlh6yFSJEms

    20 < myComponent
}

```

A <HYPERMEDIA> or <FRAGMENT> configuration can be flat or a nested

```properties
fragment = <FRAGMENT>
fragment.content = <TREE>
fragment.content {
    10 = <HTML>
    10.value = <p>THIS IS HTML</p>
}
```

```properties
fragment = <FRAGMENT>
fragment.content = <TREE>
fragment.content.10 = <HTML>
fragment.content.value = <p>THIS IS HTML</p>
```


