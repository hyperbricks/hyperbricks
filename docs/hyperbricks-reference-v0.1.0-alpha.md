
# HyperBricks
**Version:** v0.1.0-alpha  
**Build time:** 2025-01-16T23:09:22Z

Go direct to:

- [HyperBricks type reference](#hyperbricks-type-reference)
- [HyperBricks examples](#hyperbricks-examples)

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



---

## HyperBricks type reference
 

### Category: **component**

- &lt;API_RENDER&gt;
- &lt;CSS&gt;
- &lt;HTML&gt;
- &lt;IMAGE&gt;
- &lt;IMAGES&gt;
- &lt;JS&gt;
- &lt;JSON&gt;
- &lt;MENU&gt;
- &lt;PLUGIN&gt;
- &lt;TEXT&gt;


### Category: **composite**

- &lt;FRAGMENT&gt;
- &lt;HYPERMEDIA&gt;




---
### Category: **component**


#### &lt;API_RENDER&gt;

**Type Description**






















---
**Properties**

- [attributes](#api_render-attributes)
- [enclose](#api_render-enclose)
- [endpoint](#api_render-endpoint)
- [method](#api_render-method)
- [headers](#api_render-headers)
- [body](#api_render-body)
- [template](#api_render-template)
- [istemplate](#api_render-istemplate)
- [user](#api_render-user)
- [pass](#api_render-pass)


---

## api_render attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## api_render enclose
#### enclose

**Description**  
The wrapping HTML element for the header divided by |

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## api_render endpoint
#### endpoint

**Description**  
The API endpoint

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## api_render method
#### method

**Description**  
HTTP method to use for API calls, GET POST PUT DELETE etc... 

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## api_render headers
#### headers

**Description**  
Optional HTTP headers for API requests

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## api_render body
#### body

**Description**  
Use the string format of the example, do not use an nested object to define. The values will be parsed en send with the request.

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## api_render template
#### template

**Description**  
Template used for rendering API output

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## api_render istemplate
#### istemplate

**Description**  


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## api_render user
#### user

**Description**  
User for basic auth

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## api_render pass
#### pass

**Description**  
User for basic auth

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````



---

#### &lt;CSS&gt;

**Type Description**












---
**Properties**

- [attributes](#css-attributes)
- [enclose](#css-enclose)
- [inline](#css-inline)
- [link](#css-link)
- [file](#css-file)


---

## css attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## css enclose
#### enclose

**Description**  
The wrapping HTML element for the header divided by |

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## css inline
#### inline

**Description**  
Use inline to define css in a multiline block &lt;&lt;[ /* css goes here */ ]&gt;&gt;

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## css link
#### link

**Description**  
Use link for a link tag

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## css file
#### file

**Description**  
file overrides link and inline, it loads contents of a file and renders it in a style tag.

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````



---

#### &lt;HTML&gt;

**Type Description**










---
**Properties**

- [attributes](#html-attributes)
- [enclose](#html-enclose)
- [value](#html-value)
- [trimspace](#html-trimspace)


---

## html attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## html enclose
#### enclose

**Description**  
The wrapping HTML element for the header divided by |

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## html value
#### value

**Description**  
The raw HTML content

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## html trimspace
#### trimspace

**Description**  
The raw HTML content

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````



---

#### &lt;IMAGE&gt;

**Type Description**
























---
**Properties**

- [attributes](#image-attributes)
- [enclose](#image-enclose)
- [src](#image-src)
- [width](#image-width)
- [height](#image-height)
- [alt](#image-alt)
- [title](#image-title)
- [class](#image-class)
- [quality](#image-quality)
- [loading](#image-loading)
- [is_static](#image-is_static)


---

## image attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## image enclose
#### enclose

**Description**  
The wrapping HTML element for the header divided by |

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## image src
#### src

**Description**  
The source URL of the image

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## image width
#### width

**Description**  
The width of the image (can be a number or percentage)

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## image height
#### height

**Description**  
The height of the image (can be a number or percentage)

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## image alt
#### alt

**Description**  
Alternative text for the image

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## image title
#### title

**Description**  
The title attribute of the image

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## image class
#### class

**Description**  
CSS class for styling the image

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## image quality
#### quality

**Description**  
Image quality for optimization

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## image loading
#### loading

**Description**  
Lazy loading strategy (e.g., &#39;lazy&#39;, &#39;eager&#39;)

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## image is_static
#### is_static

**Description**  
Flag indicating if the image is static

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````



---

#### &lt;IMAGES&gt;

**Type Description**














---
**Properties**

- [attributes](#images-attributes)
- [enclose](#images-enclose)
- [directory](#images-directory)
- [width](#images-width)
- [height](#images-height)
- [is_static](#images-is_static)


---

## images attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## images enclose
#### enclose

**Description**  
The wrapping HTML element for the header divided by |

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## images directory
#### directory

**Description**  
The directory path containing the images

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## images width
#### width

**Description**  
The width of the images (can be a number or percentage)

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## images height
#### height

**Description**  
The height of the images (can be a number or percentage)

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## images is_static
#### is_static

**Description**  
Flag indicating if the images are static

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````



---

#### &lt;JS&gt;

**Type Description**












---
**Properties**

- [attributes](#js-attributes)
- [enclose](#js-enclose)
- [inline](#js-inline)
- [link](#js-link)
- [file](#js-file)


---

## js attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## js enclose
#### enclose

**Description**  
The wrapping HTML element for the header divided by |

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## js inline
#### inline

**Description**  
Use inline to define JavaScript in a multiline block &lt;&lt;[ /* JavaScript goes here */ ]&gt;&gt;

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## js link
#### link

**Description**  
Use link for a script tag with a src attribute

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## js file
#### file

**Description**  
File overrides link and inline, it loads contents of a file and renders it in a script tag.

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````



---

#### &lt;JSON&gt;

**Type Description**










---
**Properties**

- [attributes](#json-attributes)
- [enclose](#json-enclose)
- [file](#json-file)
- [template](#json-template)


---

## json attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## json enclose
#### enclose

**Description**  
The wrapping HTML element for the header divided by |

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## json file
#### file

**Description**  
Path to the local JSON file

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## json template
#### template

**Description**  
Template for rendering output

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````



---

#### &lt;MENU&gt;

**Type Description**


















---
**Properties**

- [attributes](#menu-attributes)
- [enclose](#menu-enclose)
- [section](#menu-section)
- [order](#menu-order)
- [sort](#menu-sort)
- [active](#menu-active)
- [item](#menu-item)
- [enclose](#menu-enclose)


---

## menu attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## menu enclose
#### enclose

**Description**  
The wrapping HTML element for the header divided by |

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## menu section
#### section

**Description**  
The section of the menu to display.

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## menu order
#### order

**Description**  
The order of items in the menu (&#39;asc&#39; or &#39;desc&#39;).

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## menu sort
#### sort

**Description**  
The field to sort menu items by (&#39;title&#39;, &#39;route&#39;, or &#39;index&#39;).

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## menu active
#### active

**Description**  
Template for the active menu item.

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## menu item
#### item

**Description**  
Template for regular menu items.

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## menu enclose
#### enclose

**Description**  
Template to wrap the menu items.

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````



---

#### &lt;PLUGIN&gt;

**Type Description**












---
**Properties**

- [attributes](#plugin-attributes)
- [enclose](#plugin-enclose)
- [plugin](#plugin-plugin)
- [classes](#plugin-classes)
- [data](#plugin-data)


---

## plugin attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## plugin enclose
#### enclose

**Description**  
The wrapping HTML element for the header divided by |

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## plugin plugin
#### plugin

**Description**  


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## plugin classes
#### classes

**Description**  
Optional CSS classes for the link

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## plugin data
#### data

**Description**  


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````



---

#### &lt;TEXT&gt;

**Type Description**








---
**Properties**

- [attributes](#text-attributes)
- [enclose](#text-enclose)
- [value](#text-value)


---

## text attributes
#### attributes

**Description**  
Extra attributes like id, data-role, data-action

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## text enclose
#### enclose

**Description**  
The wrapping HTML element for the header divided by |

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## text value
#### value

**Description**  
The paragraph content

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````



---


---
### Category: **composite**


#### &lt;FRAGMENT&gt;

**Type Description**








































---
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


---

## fragment response
#### response

**Description**  
HTMX response header configuration.

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment title
#### title

**Description**  
The title of the fragment

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment route
#### route

**Description**  
The route (URL-friendly identifier) for the fragment

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment section
#### section

**Description**  
The section the fragment belongs to

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment enclose
#### enclose

**Description**  
Wrapping property for the fragment rendered output

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment template
#### template

**Description**  
Template configurations for rendering the fragment

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment static
#### static

**Description**  
Static file path associated with the fragment

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment index
#### index

**Description**  
Index number is a sort order option for the fragment menu section. See MENU and MENU_TEMPLATE for further explanation

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment hx_location
#### hx_location

**Description**  
allows you to do a client-side redirect that does not do a full page reload

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	response {
        hx_location = someurl
    }
}

````




---

## fragment hx_push_url
#### hx_push_url

**Description**  
pushes a new url into the history stack

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment hx_redirect
#### hx_redirect

**Description**  
can be used to do a client-side redirect to a new location

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment hx_refresh
#### hx_refresh

**Description**  
if set to &#39;true&#39; the client-side will do a full refresh of the page

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment hx_replace_url
#### hx_replace_url

**Description**  
replaces the current url in the location bar

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment hx_reswap
#### hx_reswap

**Description**  
allows you to specify how the response will be swapped

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment hx_retarget
#### hx_retarget

**Description**  
a css selector that updates the target of the content update

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment hx_reselect
#### hx_reselect

**Description**  
a css selector that allows you to choose which part of the response is used to be swapped in

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment hx_trigger
#### hx_trigger

**Description**  
allows you to trigger client-side events

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment hx_trigger_after_settle
#### hx_trigger_after_settle

**Description**  
allows you to trigger client-side events after the settle step

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## fragment hx_trigger_after_swap
#### hx_trigger_after_swap

**Description**  
allows you to trigger client-side events after the swap step

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````



---

#### &lt;HYPERMEDIA&gt;

**Type Description**




























---
**Properties**

- [title](#hypermedia-title)
- [route](#hypermedia-route)
- [section](#hypermedia-section)
- [bodytag](#hypermedia-bodytag)
- [enclose](#hypermedia-enclose)
- [favicon](#hypermedia-favicon)
- [template](#hypermedia-template)
- [isstatic](#hypermedia-isstatic)
- [static](#hypermedia-static)
- [index](#hypermedia-index)
- [doctype](#hypermedia-doctype)
- [htmltag](#hypermedia-htmltag)
- [head](#hypermedia-head)


---

## hypermedia title
#### title

**Description**  
The title of the hypermedia site

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## hypermedia route
#### route

**Description**  
The route (URL-friendly identifier) for the hypermedia

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## hypermedia section
#### section

**Description**  
The section the hypermedia belongs to. This can be used with the component &lt;MENU&gt; for example.

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## hypermedia bodytag
#### bodytag

**Description**  
Special body wrap with use of |. Please note that this will not work when a &lt;HYPERMEDIA&gt;.template is configured. In that case, you have to add the bodytag in the template.

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




---

## hypermedia enclose
#### enclose

**Description**  
Enclosure of the property for the hypermedia

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## hypermedia favicon
#### favicon

**Description**  
Path to the favicon for the hypermedia

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## hypermedia template
#### template

**Description**  
Template configurations for rendering the hypermedia. See &lt;TEMPLATE&gt; for field descriptions.

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## hypermedia isstatic
#### isstatic

**Description**  


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## hypermedia static
#### static

**Description**  
Static file path associated with the hypermedia, for rendering out the hypermedia to static files.

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## hypermedia index
#### index

**Description**  
Index number is a sort order option for the hypermedia defined in the section field. See &lt;MENU&gt; for further explanation and field options

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## hypermedia doctype
#### doctype

**Description**  
Alternative Doctype for the HTML document

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## hypermedia htmltag
#### htmltag

**Description**  
The opening HTML tag with attributes

**Example**
````properties
fragment = <FRAGMENT>
fragment {
	
}

````




---

## hypermedia head
#### head

**Description**  
Configurations for the head section of the hypermedia

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
    10 = <HTML>
    10.value = <meta name="generator" content="hyperbrickszzzz cms">
     
}
hypermedia.10 = <HTML>
hypermedia.10.value = <p>some HTML</p>

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <head>
    <meta name="generator" content="hyperbrickszzzz cms">
    <meta name="generator" content="hyperbricks cms">
    <meta name="a" content="b">
    <meta name="b" content="c">
    <link rel="stylesheet" href="styles.css">
    <link rel="stylesheet" href="xxxx">
    <script src="styles.css"></script>
    <script src="xxxx"></script>
  </head>
  <body>
    <p>
      some HTML
    </p>
  </body>
</html>
````



---


