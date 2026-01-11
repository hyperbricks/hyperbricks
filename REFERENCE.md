**Licence:** MIT  
**Version:** v0.7.0-alpha  
**Build time:** 2026-01-11T12:52:10Z

## Build Status

[![Build & Test (develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml/badge.svg?branch=develop)](https://github.com/hyperbricks/hyperbricks/actions/workflows/ci-all-tests.yml?query=branch%3Adevelop)

## HyperBricks type reference



# Category: **component**





## &lt;HTML&gt;

**Type Description**






Component for rendering all your single or multiline snippets.


**Main Example**
````properties
html = <HTML>
html.value = <<[
  <p>HTML TEST</p>
]>>
html.enclose = <div>|</div>

````


**Expected Result**
````html
<div>
  <p>
    HTML TEST
  </p>
</div>
````


**more**










**Properties**





### enclose

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














### value

**Description**  
The raw HTML content


**Example**
````properties
html = <HTML>
html.value = <p>HTML TEST</p>

````

**Expected Result**

````html
<p>
  HTML TEST
</p>
````











### trimspace

**Description**  
Property trimspace filters (if set to true true),  all leading and trailing white space removed, as defined by Unicode.


**Example**
````properties
html = <HTML>
html.value = <<[
        <p>HTML TEST</p>    
    ]>>
html.trimspace = true

````

**Expected Result**

````html
<p>
  HTML TEST
</p>
````













## &lt;PLUGIN&gt;

**Type Description**














**Properties**





### attributes

**Description**  
Extra attributes like id, data-role, data-action


**Example**
````properties
plugin = <PLUGIN>
plugin {
    plugin = example
    attributes {
        data-role = demo
    }
}

````

**Expected Result**

````html
<!-- Error loading plugin example: plugin.Open("bin/plugins/example.so"): realpath failed -->
````











### enclose

**Description**  
The enclosing HTML element for the header divided by |


**Example**
````properties
plugin = <PLUGIN>
plugin {
    plugin = example
    enclose = <div>|</div>
}

````

**Expected Result**

````html
<!-- Error loading plugin example: plugin.Open("bin/plugins/example.so"): realpath failed -->
````











### plugin

**Description**  



**Example**
````properties
plugin = <PLUGIN>
plugin {
    plugin = example
}

````

**Expected Result**

````html
<!-- Error loading plugin example: plugin.Open("bin/plugins/example.so"): realpath failed -->
````











### classes

**Description**  
Optional CSS classes for the link


**Example**
````properties
plugin = <PLUGIN>
plugin {
    plugin = example
    classes = [primary, secondary]
}

````

**Expected Result**

````html
<!-- Error loading plugin example: plugin.Open("bin/plugins/example.so"): realpath failed -->
````











### data

**Description**  



**Example**
````properties
plugin = <PLUGIN>
plugin {
    plugin = example
    data {
        key = value
    }
}

````

**Expected Result**

````html
<!-- Error loading plugin example: plugin.Open("bin/plugins/example.so"): realpath failed -->
````













## &lt;TEXT&gt;

**Type Description**









**Main Example**
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


**more**








**Properties**





### enclose

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














### value

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












# Category: **composite**





## &lt;API_FRAGMENT_RENDER&gt;

**Type Description**






































A &lt;FRAGMENT&gt; dynamically renders a part of an HTML page, allowing updates without a full page reload and improving performance and user experience.


**Main Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
}

````



**more**
















**Properties**





### endpoint

**Description**  
The API endpoint


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/endpoint
    method = GET
    route = api-fragment
}

````









### method

**Description**  
HTTP method to use for API calls, GET POST PUT DELETE etc... 


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = POST
    route = api-fragment
}

````









### headers

**Description**  
Optional HTTP headers for API requests


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    headers {
        Accept = application/json
    }
}

````









### body

**Description**  
Use the string format of the example, do not use an nested object to define. The values will be parsed en send with the request.


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    body = {"foo":"bar"}
}

````









### template

**Description**  
Loads contents of a template file in the modules template directory


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    template = example
}

````

**Expected Result**

````html
[error parsing template]
````











### inline

**Description**  
Use inline to define the template in a multiline block &lt;&lt;[ /* Template goes here */ ]&gt;&gt;


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    inline = <div>INLINE</div>
}

````

**Expected Result**

````html
<div>
  INLINE
</div>
````











### values

**Description**  
Key-value pairs for template rendering


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    values {
        foo = bar
    }
}

````









### username

**Description**  
Username for basic auth


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    username = user1
}

````









### password

**Description**  
Password for basic auth


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    password = secret
}

````









### setcookie

**Description**  
Set template for cookie


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    setcookie = session=abc
}

````









### querykeys

**Description**  
Set allowed proxy query keys


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    querykeys = [foo, bar]
}

````









### queryparams

**Description**  
Set proxy query key in the confifuration


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    queryparams {
        foo = bar
    }
}

````









### jwtsecret

**Description**  
When not empty it uses jwtsecret for Bearer Token Authentication. When empty it switches if configured to basic auth via http.Request


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    jwtsecret = secret
}

````









### jwtclaims

**Description**  
jwt claim map


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    jwtclaims {
        sub = user
    }
}

````









### debug

**Description**  
Debug the response data


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    debug = true
    debugpanel = true
}

````









### debugpanel

**Description**  
Debug the response data


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    debug = true
    debugpanel = true
}

````









### response

**Description**  
HTMX response header configuration.


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    response {
        hx_trigger = myEvent
    }
}

````












### title

**Description**  
The title of the fragment


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    title = API Fragment Title
}

````









### route

**Description**  
The route (URL-friendly identifier) for the fragment


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment-route
}

````









### section

**Description**  
The section the fragment belongs to


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    section = api
}

````









### enclose

**Description**  
Wrapping property for the fragment rendered output


**Example**
````properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment {
    endpoint = https://example.com/fragment
    method = GET
    route = api-fragment
    enclose = <div>|</div>
}

````

**Expected Result**

````html
<div></div>
````











### index

**Description**  
Index number is a sort order option for the &lt;MENU&gt; section. See &lt;MENU&gt; for further explanation


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	index = 1
}

````










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


## &lt;FRAGMENT&gt;

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





### response

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










### beautify

**Description**  
Override server.beautify for this object when rendered directly


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	beautify = false
}

````









### title

**Description**  
The title of the fragment, only used in the context of the &lt;MENU&gt; component. For document title use &lt;HYPERMEDIA&gt; type.


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	title = Some Title
}

````









### route

**Description**  
The route (URL-friendly identifier) for the fragment


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	route = index
}

````









### section

**Description**  
The section the fragment belongs to. This can be used with the component &lt;MENU&gt; for example.


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	section = some_section
}

````









### enclose

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











### template

**Description**  
Template configurations for rendering the fragment. (This will disable rendering any content added to the alpha numeric items that are added to the fragment root object.) See &lt;TEMPLATE&gt; for more details using templates.


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	template {
        
        inline = <<[
            <div>{{.content}}</div>

        ]>>
      
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











### static

**Description**  
Static file path associated with the fragment, this will only work for a hx-get (GET) request. 


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	static = some_static_file.extension
}

````









### cache

**Description**  
Cache expire string


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = index
hypermedia.cache = 10m
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












### nocache

**Description**  
Explicitly disable cache


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = index
hypermedia.nocache = true
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












### index

**Description**  
Index number is a sort order option for the &lt;MENU&gt; section. See &lt;MENU&gt; for further explanation


**Example**
````properties
fragment = <FRAGMENT>
fragment {
	index = 1
}

````









### content_type

**Description**  
content type header definition


**Example**
````properties
fragment = <FRAGMENT>
fragment.content_type = text/json 

````









### hx_location

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









### hx_push_url

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









### hx_redirect

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









### hx_refresh

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









### hx_replace_url

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









### hx_reswap

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









### hx_retarget

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









### hx_reselect

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









### hx_trigger

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









### hx_trigger_after_settle

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









### hx_trigger_after_swap

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











## &lt;HEAD&gt;

**Type Description**














**Properties**





### title

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











### favicon

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











### meta

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











### css

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











### js

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













## &lt;HYPERMEDIA&gt;

**Type Description**




HYPERMEDIA type is the main initiator of a htmx document. Its location is defined by the route property. Use &lt;FRAGMENT&gt; to utilize hx-[method] (GET,POST etc) requests.  


**Main Example**
````properties
css = <HTML>
css.value = <<[
    <style>
        body {
            padding:20px;
        }
    </style>
]>>



hypermedia = <HYPERMEDIA>
hypermedia.head = <HEAD>
hypermedia.head {
    10 < css
    20 = <CSS>
    20.inline = <<[
        .content {
            color:green;
        }
    ]>>
}
hypermedia.10 = <TREE>
hypermedia.10 {
    1 = <HTML>
    1.value = <p>SOME CONTENT</p>
}


````


**Expected Result**
````html
<!DOCTYPE html>
<html>
  <head>
    <style>
      body {
      padding:20px;
      }
    </style>
    <style>
      .content {
      color:green;
      }
    </style>
    <meta name="generator" content="hyperbricks cms">
  </head>
  <body>
    <p>
      SOME CONTENT
    </p>
  </body>
</html>
````


**more**







































**Properties**








### beautify

**Description**  
Override server.beautify for this object when rendered directly


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia {
    beautify = false
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











### title

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











### route

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











### section

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











### bodytag

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












### enclose

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












### favicon

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












### template

**Description**  
Template configurations for rendering the hypermedia. See &lt;TEMPLATE&gt; for field descriptions.


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia {
	template {
        
        inline = <<[
            <div>{{.content}}</div>
        ]>>

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











### cache

**Description**  
Cache expire string


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = index
hypermedia.cache = 10m
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












### nocache

**Description**  
Explicitly disable cache


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia.route = index
hypermedia.nocache = true
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












### static

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











### index

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











### doctype

**Description**  
Alternative Doctype for the HTML document


**Example**
````properties
hypermedia = <HYPERMEDIA>

hypermedia.doctype = <!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">

````

**Expected Result**

````html
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html>
  <body></body>
</html>
````











### htmltag

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











### head

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
    999.value = <!-- 999 overrides default generator meta tag -->

    1001 = <CSS>
    1001.inline = <<[
        body {
          padding:10px;
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
    <!-- 999 overrides default generator meta tag -->
    <meta name="a" content="b">
    <meta name="b" content="c">
    <link rel="stylesheet" href="styles.css">
    <link rel="stylesheet" href="xxxx">
    <script src="styles.css"></script>
    <script src="xxxx"></script>
    <style>
      body {
      padding:10px;
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











### content_type

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













## &lt;TEMPLATE&gt;

**Type Description**




&lt;TEMPLATE&gt; can be used nested in &lt;FRAGMENT&gt; or &lt;HYPERMEDIA&gt; types. It uses golang&#39;s standard html/template library.


**Main Example**
````properties

template = youtube.tmpl


inline = <<[
    <iframe width="{{.width}}" height="{{.height}}" src="{{.src}}"></iframe>
]>>

myComponent = <TEMPLATE>
myComponent {
    inline = <<[
        <iframe width="{{.width}}" height="{{.height}}" src="{{.src}}"></iframe>
    ]>>
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


**more**


















**Properties**








### template

**Description**  
The template used for rendering.


**Example**
````properties
myComponent = <TEMPLATE>
myComponent {

    
    inline = <<[
        <iframe width="{{.width}}" height="{{.height}}" src="{{.src}}"></iframe>
    ]>>
  
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











### inline

**Description**  
The inline template used for rendering.


**Example**
````properties
myComponent = <TEMPLATE>
myComponent {
    
    inline = <<[
        <iframe width="{{.width}}" height="{{.height}}" src="{{.src}}"></iframe>
    ]>>
  
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











### querykeys

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









### queryparams

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









### values

**Description**  
Key-value pairs for template rendering


**Example**
````properties

$test = hello world

myComponent = <TEMPLATE>
myComponent {
    inline = <<[
        <h1>{{.header}}</h1>
        <p>{{.text}}</p>
    ]>>

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











### enclose

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













## &lt;TREE&gt;

**Type Description**




TREE description


**Main Example**
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
    }
}

````


**Expected Result**
````html
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
````


**more**








**Properties**








### enclose

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












# Category: **data**





## &lt;API_RENDER&gt;

**Type Description**




































&lt;API_RENDER&gt; description


**Main Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
}

````



**more**






**Properties**





### enclose

**Description**  
The enclosing HTML element for the header divided by |


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    enclose = <div>|</div>
}

````

**Expected Result**

````html
<div></div>
````











### endpoint

**Description**  
The API endpoint


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/endpoint
    method = GET
}

````









### method

**Description**  
HTTP method to use for API calls, GET POST PUT DELETE etc... 


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = POST
}

````









### headers

**Description**  
Optional HTTP headers for API requests


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    headers {
        Accept = application/json
    }
}

````









### body

**Description**  
Use the string format of the example, do not use an nested object to define. The values will be parsed en send with the request.


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    body = {"foo":"bar"}
}

````









### template

**Description**  
Loads contents of a template file in the modules template directory


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    template = example
}

````

**Expected Result**

````html
[error parsing template]
````











### inline

**Description**  
Use inline to define the template in a multiline block &lt;&lt;[ /* Template goes here */ ]&gt;&gt;


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    inline = <div>INLINE</div>
}

````

**Expected Result**

````html
<div>
  INLINE
</div>
````











### values

**Description**  
Key-value pairs for template rendering


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    values {
        foo = bar
    }
}

````









### username

**Description**  
Username for basic auth


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    username = user1
}

````









### password

**Description**  
Password for basic auth


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    password = secret
}

````









### querykeys

**Description**  
Set allowed proxy query keys


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    querykeys = [foo, bar]
}

````









### queryparams

**Description**  
Set proxy query key in the confifuration


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    queryparams {
        foo = bar
    }
}

````









### jwtsecret

**Description**  
When not empty it uses jwtsecret for Bearer Token Authentication. When empty it switches if configured to basic auth via http.Request


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    jwtsecret = secret
}

````









### jwtclaims

**Description**  
jwt claim map


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    jwtclaims {
        sub = user
    }
}

````









### debug

**Description**  
Debug the response data


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    debug = true
    debugpanel = true
}

````









### debugpanel

**Description**  
Debug the response data


**Example**
````properties
api_render = <API_RENDER>
api_render {
    endpoint = https://example.com/api
    method = GET
    debug = true
    debugpanel = true
}

````














## &lt;JSON&gt;

**Type Description**








Debug the response data


**Main Example**
````properties
local_json_test = <JSON_RENDER>
local_json_test {
	file =  hyperbricks-test-files/assets/quotes.json

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


**more**
















**Properties**





### attributes

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











### enclose

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














### file

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











### template

**Description**  
Template for rendering output


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











### inline

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











### values

**Description**  
Key-value pairs for template rendering


**Example**
````properties
local_json_test = <JSON_RENDER>
local_json_test {
	file =  hyperbricks-test-files/assets/quotes.json

    
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











### debug

**Description**  
Debug the response data


**Example**
````properties
local_json_test = <JSON_RENDER>
local_json_test {
	file =  hyperbricks-test-files/assets/quotes.json

    
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












# Category: **menu**





## &lt;MENU&gt;

**Type Description**


















**Properties**





### enclose

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











### section

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











### order

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











### sort

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











### active

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











### item

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











### enclose

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












# Category: **resources**





## &lt;CSS&gt;

**Type Description**








### Inline css example
&lt;div id=&#34;python&#34; class=&#34;tab-content&#34;&gt;
&lt;pre&gt;&lt;code class=&#34;language-html&#34;&gt;
&lt;p&gt;oki&lt;/p&gt;
&lt;/code&gt;&lt;/pre&gt;
&lt;/div&gt;

&lt;div id=&#34;javascript&#34; class=&#34;tab-content&#34; style=&#34;display:none;&#34;&gt;
&lt;pre&gt;&lt;code class=&#34;language-hyperbricks&#34;&gt;
css = &lt;CSS&gt;
css.file = hyperbricks-test-files/assets/styles.css
css.attributes {
    media = screen
}
css.enclose = &lt;style media=&#34;print&#34;&gt;|&lt;/style&gt;
&lt;/code&gt;&lt;/pre&gt;
&lt;/div&gt;





**Main Example**
````properties
css = <CSS>
css.file = hyperbricks-test-files/assets/styles.css
css.attributes {
    media = screen
}
css.enclose = <style media="print">|</style>

````


**Expected Result**
````html
<style media="print">
  body {
  background-color: red;
  }
</style>
````


**more**
And some other details we do not want to forget....

like an extra example:

```hyperbricks
css = &lt;CSS&gt;
css.file = hyperbricks-test-files/assets/styles.css
css.attributes {
    media = screen
}
css.enclose = &lt;style media=&#34;print&#34;&gt;|&lt;/style&gt;
```












**Properties**





### attributes

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











### enclose

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














### inline

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











### link

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











### file

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













## &lt;IMAGE&gt;

**Type Description**











**Main Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 100

image.title = Some Cute Cat!
image.class = class-a class-b class-c
image.attributes {
  usemap = #catmap 
}
image.alt = cat but cute
image.quality = 90
image.id = #cat

````


**Expected Result**
````html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" alt="cat but cute" title="Some Cute Cat!" class="class-a class-b class-c" id="#cat" usemap="#catmap" />
````


**more**
























**Properties**





### attributes

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











### enclose

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














### src

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











### width

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











### height

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











### alt

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











### title

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











### id

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











### class

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











### quality

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











### loading

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













## &lt;IMAGES&gt;

**Type Description**











**Main Example**
````properties
image = <IMAGE>
image.src = hyperbricks-test-files/assets/cute_cat.jpg
image.width = 100

image.title = Some Cute Cat!
image.class = class-a class-b class-c
image.attributes {
  usemap = #catmap 
}
image.alt = cat but cute
image.quality = 90
image.id = #cat

````


**Expected Result**
````html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" alt="cat but cute" title="Some Cute Cat!" class="class-a class-b class-c" id="#cat" usemap="#catmap" />
````


**more**
























**Properties**





### attributes

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











### enclose

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














### directory

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











### width

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











### height

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











### id

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











### class

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











### alt

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











### title

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











### quality

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











### loading

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













## &lt;JS&gt;

**Type Description**








Extra attributes like id, data-role, data-action, type


**Main Example**
````properties
js = <JAVASCRIPT>
js.file = hyperbricks-test-files/assets/script.js
js.attributes {
    type = text/javascript
}

````


**Expected Result**
````html
<script type="text/javascript">
  console.log("Hello World!")
</script>
````


**more**












**Properties**





### attributes

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











### enclose

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














### inline

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











### link

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
    <script src="hyperbricks-test-files/assets/main.js" type="text/javascript"></script>
    <meta name="generator" content="hyperbricks cms">
  </head>
  <body></body>
</html>
````











### file

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











