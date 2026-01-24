Plugin that returns an inherited template map after applying bind-based updates.

**Example config:**
```
article = <TREE>
article {
  10 = <TEXT>
  10.@bind = title
  10.value = Hello World
}

inherit_article = <PLUGIN>
inherit_article.plugin = InheritMapPlugin__test-004-plugins@1.0.0
inherit_article.data.template < article
inherit_article.data.title = Updated title
```
