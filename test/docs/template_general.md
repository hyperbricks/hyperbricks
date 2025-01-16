## HyperBricks Documentation

HyperBricks is a powerful CMS that use nested declarative configuration files, enabling the rapid development of [htmx](https://htmx.org/) hypermedia-based applications.

This declarative configuration files (hyperbricks) allows to declare and describe state of a document.

Hypermedia documents or fragments are declared with:
&lt;HYPERMEDIA&gt; or &lt;FRAGMENT&gt;

### basic example ###
```properties
fragment = <FRAGMENT>
fragment.content = <TREE>
fragment.content {
    10 = <HTML>
    10.value = <p>THIS IS HTML</p>

    20 = <HTML>
    20.value = <p>THIS IS HTML</p>
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


