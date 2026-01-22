**Licence:** MIT  
**Version:** v0.7.7-alpha  
**Build time:** 2026-01-22T15:36:50Z

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










   # HyperBricks API-RENDER 

HyperBricks renders HTML directly from APIs. Use `<API_RENDER>` for cacheable/public data. Use `<API_FRAGMENT_RENDER>` for live, interactive, or authenticated fragments (HTMX-style partials). Fragments are always dynamic.

---

## Components at a glance

| Component               | Primary use                          | Cache       | Client auth | Typical cases                   |
| ----------------------- | ------------------------------------ | ----------- | ----------- | ------------------------------- |
| `<API_RENDER>`          | Public, cacheable API → HTML         | Optional    | No          | Public widgets, feeds           |
| `<API_FRAGMENT_RENDER>` | Interactive/auth API → HTML fragment | No (forced) | Yes         | Login, dashboards, HTMX islands |

**IMPORTANT:** `<API_FRAGMENT_RENDER>` forces `nocache = true` at runtime.

---

## Features

### `<API_RENDER>`

* Nested and optionally cached; lives inside a composite like `<FRAGMENT>` or `<HYPERMEDIA>` so, <API_RENDER> is unlike <API_FRAGMENT_RENDER> not a route by itself; it must be included inside a root component (<HYPERMEDIA>/<FRAGMENT>).
* Fetches API data, transforms via `inline`/`template`
* Forwards only allow-listed query params (`querykeys`)
* Can set upstream request headers and modify request body
* Can apply upstream auth (JWT, Basic, cookies)

### `<API_FRAGMENT_RENDER>`

* Custom route; renders to an HTML fragment for HTMX/partial updates
* Bi-directional proxy: filters queries, forwards form/body
* Can apply upstream auth (JWT, Basic, cookies)
* HTMX response headers via `response { ... }`
* `setcookie` sets client cookie based on response data when `.Status == 200`

---

###  **Relevant Documentation**
* See the [TaskManager repository](https://github.com/hyperbricks/taskmanager/blob/main/modules/taskmanager/hyperbricks/lib/tasklist.hyperbricks#:~:text=tasklist.-,hyperbricks,-taskmanager.hyperbricks) for an example with with [PostgREST](https://postgrest.org/) and [HTMX](https://htmx.org/).
* For latest hyperbricks configuration examples see [test/dedicated/api-tests](https://github.com/hyperbricks/hyperbricks/tree/main/test/dedicated/api-tests#:~:text=api%2D-,tests,-api%2Dfragment%2Drender)
* [HTMX Out-of-Band Swaps](https://htmx.org/attributes/hx-swap-oob/)
* [HTMX Response Headers](https://htmx.org/reference/#response_headers)
* [Hypermedia Systems](https://hypermedia.systems/book/contents/)
* [Go html/template](https://pkg.go.dev/html/template)
* [Sprig Template Functions](https://masterminds.github.io/sprig/)

<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" contentStyleType="text/css" data-diagram-type="SEQUENCE"  preserveAspectRatio="none" style="background:#FFFFFF;" version="1.1" viewBox="0 0 1502 564" width="100%" zoomAndPan="magnify"><title>Direct HTMX OOB via HyperBricks &lt;API_FRAGMENT_RENDER&gt;</title><defs/><g><text fill="#000000" font-family="Verdana" font-size="22" font-weight="bold" lengthAdjust="spacing" textLength="765.0049" x="370.8447" y="35.4209">Direct HTMX OOB via HyperBricks &lt;API_FRAGMENT_RENDER&gt;</text><rect fill="#DDEEFF" height="505.7188" style="stroke:#000000;stroke-width:1;" width="464.8682" x="11" y="52.6094"/><text fill="#000000" font-family="Verdana" font-size="13" font-weight="bold" lengthAdjust="spacing" textLength="119.8247" x="183.5217" y="64.6763">Client (UI Layer)</text><rect fill="#E8F5E9" height="505.7188" style="stroke:#000000;stroke-width:1;" width="768.3896" x="589.375" y="52.6094"/><text fill="#000000" font-family="Verdana" font-size="13" font-weight="bold" lengthAdjust="spacing" textLength="198.7388" x="874.2004" y="64.6763">Server (HyperBricks Layer)</text><rect fill="#FFF8E1" height="505.7188" style="stroke:#000000;stroke-width:1;" width="126.9297" x="1369.7646" y="52.6094"/><text fill="#000000" font-family="Verdana" font-size="13" font-weight="bold" lengthAdjust="spacing" textLength="119.7612" x="1373.3489" y="64.6763">External Service</text><g><title>.API_FRAGMENT_RENDER.</title><rect fill="#FFFFFF" height="145.6641" style="stroke:#000000;stroke-width:1;" width="10" x="875.0107" y="192.4375"/></g><g><title>User</title><rect fill="#000000" fill-opacity="0.00000" height="420.9922" width="8" x="43.9551" y="103.0391"/><line style="stroke:#000000;stroke-width:1;stroke-dasharray:5,5;" x1="47" x2="47" y1="103.0391" y2="524.0313"/></g><g><title>Browser . HTMX</title><rect fill="#000000" fill-opacity="0.00000" height="420.9922" width="8" x="225.4722" y="103.0391"/><line style="stroke:#000000;stroke-width:1;stroke-dasharray:5,5;" x1="228.7695" x2="228.7695" y1="103.0391" y2="524.0313"/></g><g><title>DOM</title><rect fill="#000000" fill-opacity="0.00000" height="420.9922" width="8" x="433.9287" y="103.0391"/><line style="stroke:#000000;stroke-width:1;stroke-dasharray:5,5;" x1="436.9893" x2="436.9893" y1="103.0391" y2="524.0313"/></g><g><title>Router</title><rect fill="#000000" fill-opacity="0.00000" height="420.9922" width="8" x="629.8872" y="103.0391"/><line style="stroke:#000000;stroke-width:1;stroke-dasharray:5,5;" x1="633.375" x2="633.375" y1="103.0391" y2="524.0313"/></g><g><title>.API_FRAGMENT_RENDER.</title><rect fill="#000000" fill-opacity="0.00000" height="420.9922" width="8" x="876.0107" y="103.0391"/><line style="stroke:#000000;stroke-width:1;stroke-dasharray:5,5;" x1="879.9287" x2="879.9287" y1="103.0391" y2="524.0313"/></g><g><title>Template Engine .Go html.template . Sprig.</title><rect fill="#000000" fill-opacity="0.00000" height="420.9922" width="8" x="1175.4287" y="103.0391"/><line style="stroke:#000000;stroke-width:1;stroke-dasharray:5,5;" x1="1179.0928" x2="1179.0928" y1="103.0391" y2="524.0313"/></g><g><title>External API</title><rect fill="#000000" fill-opacity="0.00000" height="420.9922" width="8" x="1429.2295" y="103.0391"/><line style="stroke:#000000;stroke-width:1;stroke-dasharray:5,5;" x1="1432.7646" x2="1432.7646" y1="103.0391" y2="524.0313"/></g><g class="participant participant-head" data-participant="User"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="45.9102" x="25" y="71.7422"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="31.9102" x="32" y="91.7373">User</text></g><g class="participant participant-tail" data-participant="User"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="45.9102" x="25" y="523.0313"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="31.9102" x="32" y="543.0264">User</text></g><g class="participant participant-head" data-participant="HTMX"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="125.4053" x="166.7695" y="71.7422"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="111.4053" x="173.7695" y="91.7373">Browser / HTMX</text></g><g class="participant participant-tail" data-participant="HTMX"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="125.4053" x="166.7695" y="523.0313"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="111.4053" x="173.7695" y="543.0264">Browser / HTMX</text></g><g class="participant participant-head" data-participant="DOM"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="47.8789" x="413.9893" y="71.7422"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="33.8789" x="420.9893" y="91.7373">DOM</text></g><g class="participant participant-tail" data-participant="DOM"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="47.8789" x="413.9893" y="523.0313"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="33.8789" x="420.9893" y="543.0264">DOM</text></g><g class="participant participant-head" data-participant="Router"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="61.0244" x="603.375" y="71.7422"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="47.0244" x="610.375" y="91.7373">Router</text></g><g class="participant participant-tail" data-participant="Router"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="61.0244" x="603.375" y="523.0313"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="47.0244" x="610.375" y="543.0264">Router</text></g><g class="participant participant-head" data-participant="APIFR"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="210.1641" x="774.9287" y="71.7422"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="196.1641" x="781.9287" y="91.7373">&lt;API_FRAGMENT_RENDER&gt;</text></g><g class="participant participant-tail" data-participant="APIFR"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="210.1641" x="774.9287" y="523.0313"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="196.1641" x="781.9287" y="543.0264">&lt;API_FRAGMENT_RENDER&gt;</text></g><g class="participant participant-head" data-participant="Tpl"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="328.6719" x="1015.0928" y="71.7422"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="314.6719" x="1022.0928" y="91.7373">Template Engine (Go html/template + Sprig)</text></g><g class="participant participant-tail" data-participant="Tpl"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="328.6719" x="1015.0928" y="523.0313"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="314.6719" x="1022.0928" y="543.0264">Template Engine (Go html/template + Sprig)</text></g><g class="participant participant-head" data-participant="API"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="98.9297" x="1383.7646" y="71.7422"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="84.9297" x="1390.7646" y="91.7373">External API</text></g><g class="participant participant-tail" data-participant="API"><rect fill="#FFFFFF" height="30.2969" rx="2.5" ry="2.5" style="stroke:#000000;stroke-width:1;" width="98.9297" x="1383.7646" y="523.0313"/><text fill="#000000" font-family="Verdana" font-size="14" lengthAdjust="spacing" textLength="84.9297" x="1390.7646" y="543.0264">External API</text></g><g><title>.API_FRAGMENT_RENDER.</title><rect fill="#FFFFFF" height="145.6641" style="stroke:#000000;stroke-width:1;" width="10" x="875.0107" y="192.4375"/></g><g class="message" data-participant-1="User" data-participant-2="HTMX"><polygon fill="#000000" points="217.4722,130.1719,227.4722,134.1719,217.4722,138.1719,221.4722,134.1719" style="stroke:#000000;stroke-width:1;"/><line style="stroke:#000000;stroke-width:1;" x1="47.9551" x2="223.4722" y1="134.1719" y2="134.1719"/><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="157.5171" x="54.9551" y="129.106">click / event (hx-trigger)</text></g><g class="message" data-participant-1="HTMX" data-participant-2="Router"><polygon fill="#000000" points="621.8872,159.3047,631.8872,163.3047,621.8872,167.3047,625.8872,163.3047" style="stroke:#000000;stroke-width:1;"/><line style="stroke:#000000;stroke-width:1;" x1="229.4722" x2="627.8872" y1="163.3047" y2="163.3047"/><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="380.415" x="236.4722" y="158.2388">HTTP hx-get|hx-post /api_fragment_oob (HX-Request:true)</text></g><g class="message" data-participant-1="Router" data-participant-2="APIFR"><polygon fill="#000000" points="863.0107,188.4375,873.0107,192.4375,863.0107,196.4375,867.0107,192.4375" style="stroke:#000000;stroke-width:1;"/><line style="stroke:#000000;stroke-width:1;" x1="633.8872" x2="869.0107" y1="192.4375" y2="192.4375"/><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="222.1235" x="640.8872" y="187.3716">dispatch route=api_fragment_oob</text></g><g class="message" data-participant-1="APIFR" data-participant-2="API"><polygon fill="#000000" points="1421.2295,217.5703,1431.2295,221.5703,1421.2295,225.5703,1425.2295,221.5703" style="stroke:#000000;stroke-width:1;"/><line style="stroke:#000000;stroke-width:1;" x1="885.0107" x2="1427.2295" y1="221.5703" y2="221.5703"/><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="221.5205" x="892.0107" y="216.5044">fetch API data with authentication</text></g><g class="message" data-participant-1="API" data-participant-2="APIFR"><polygon fill="#000000" points="896.0107,246.7031,886.0107,250.7031,896.0107,254.7031,892.0107,250.7031" style="stroke:#000000;stroke-width:1;"/><line style="stroke:#000000;stroke-width:1;stroke-dasharray:2,2;" x1="890.0107" x2="1432.2295" y1="250.7031" y2="250.7031"/><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="94.6499" x="902.0107" y="245.6372">200 JSON/data</text></g><g class="message" data-participant-1="APIFR" data-participant-2="Tpl"><polygon fill="#000000" points="1167.4287,275.8359,1177.4287,279.8359,1167.4287,283.8359,1171.4287,279.8359" style="stroke:#000000;stroke-width:1;"/><line style="stroke:#000000;stroke-width:1;" x1="885.0107" x2="1173.4287" y1="279.8359" y2="279.8359"/><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="145.9326" x="892.0107" y="274.77">render data into HTML</text></g><g class="message" data-participant-1="Tpl" data-participant-2="APIFR"><polygon fill="#000000" points="896.0107,304.9688,886.0107,308.9688,896.0107,312.9688,892.0107,308.9688" style="stroke:#000000;stroke-width:1;"/><line style="stroke:#000000;stroke-width:1;stroke-dasharray:2,2;" x1="890.0107" x2="1178.4287" y1="308.9688" y2="308.9688"/><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="153.6514" x="902.0107" y="303.9028">HTML with OOB &lt;div&gt;s</text></g><g class="message" data-participant-1="APIFR" data-participant-2="HTMX"><polygon fill="#000000" points="240.4722,334.1016,230.4722,338.1016,240.4722,342.1016,236.4722,338.1016" style="stroke:#000000;stroke-width:1;"/><line style="stroke:#000000;stroke-width:1;stroke-dasharray:2,2;" x1="234.4722" x2="879.0107" y1="338.1016" y2="338.1016"/><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="176.3506" x="246.4722" y="333.0356">200 HTML with HX headers</text></g><path d="M885,351.1016 L885,421.1016 L1319,421.1016 L1319,361.1016 L1309,351.1016 L885,351.1016" fill="#FFFFFF" style="stroke:#000000;stroke-width:1;"/><path d="M1309,351.1016 L1309,361.1016 L1319,361.1016 L1309,351.1016" fill="#FFFFFF" style="stroke:#000000;stroke-width:1;"/><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="155.5747" x="891" y="368.1685">HX-Trigger: dataLoaded</text><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="96.3574" x="891" y="383.3013">Body contains:</text><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="348.8354" x="891" y="398.4341">&lt;div id="title" hx-swap-oob="true"&gt;New Title&lt;/div&gt;</text><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="413.6895" x="891" y="413.5669">&lt;div id="list" hx-swap-oob="beforeend"&gt;&lt;li&gt;Item&lt;/li&gt;&lt;/div&gt;</text><g class="message" data-participant-1="HTMX" data-participant-2="DOM"><polygon fill="#000000" points="425.9287,443.7656,435.9287,447.7656,425.9287,451.7656,429.9287,447.7656" style="stroke:#000000;stroke-width:1;"/><line style="stroke:#000000;stroke-width:1;" x1="229.4722" x2="431.9287" y1="447.7656" y2="447.7656"/><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="184.4565" x="236.4722" y="442.6997">apply in-target swap (if any)</text></g><g class="message" data-participant-1="HTMX" data-participant-2="DOM"><polygon fill="#000000" points="425.9287,472.8984,435.9287,476.8984,425.9287,480.8984,429.9287,476.8984" style="stroke:#000000;stroke-width:1;"/><line style="stroke:#000000;stroke-width:1;" x1="229.4722" x2="431.9287" y1="476.8984" y2="476.8984"/><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="113.8198" x="236.4722" y="471.8325">apply OOB swaps</text></g><g class="message" data-participant-1="HTMX" data-participant-2="User"><polygon fill="#000000" points="58.9551,502.0313,48.9551,506.0313,58.9551,510.0313,54.9551,506.0313" style="stroke:#000000;stroke-width:1;"/><line style="stroke:#000000;stroke-width:1;stroke-dasharray:2,2;" x1="52.9551" x2="228.4722" y1="506.0313" y2="506.0313"/><text fill="#000000" font-family="Verdana" font-size="13" lengthAdjust="spacing" textLength="114.5688" x="64.9551" y="500.9653">fire "dataLoaded"</text></g><!--SRC=[RPJRRjf048RlzoccxWseGauhLKejOYaKEr05YC2Hk4EBFU1LZTTT3oSf3z_PvOnoPMldyF_pxTXVACSLGbL8LGfIL21qbS6Ke9SCfZ0QTM2Z9FJs5PgEKUdV6jhFR_rPF7v-6KJ3P3QEXjre70enrplmVXSAIuB6UnzUvHvDquEltMuKnR6ef26Lgafo_Br6StFWTOpUyY7uJjS3MRkNswJkQE0Y_1HOPi2IHzq9cWrNDwvzdWr4Z_7FwDTDgx5Uqxs5J-ToUXo8nxV92QwO6I54vLAL28qN3Jcj2fzEWMymgOnQDbs7f2hk74SxDb3A0gnrbIBxZEFuCVf-gtDEOmydBvTEbYEqGeSgWBJWkPaWRjmTvBiMiO4bGn3kCZdnC7V01SaRSC8IwOCVbQu9V5cfsSB8vOhBPrhF6UUqDSP_Qwmf8BF6fX271hQLWn90fkfAKfg3iP6d-nv2fgGsbiS1ed2FOtq02xIb_0gP90bRHJd8DUzaoGWaALtQ0cXTz7uyFC2VoMZesuAhTamu0CDas9thfKhI5iohuD1r1tjsKBBRtSxK9gpFZSumhSubipU572NthaMfUHZccXFenN4gvfGSK0TE_5LpRYOwEjj3galYZwiEX9K2bJrvxZlpBFzwAwiMO-8pvkk5Gzn2OgDjt_gwb1IgfhkAEIwqGPxO2zk52lkyUMsVNuwbTaciLv7X_HsSje6_em1aLHLxfjKYQNWTm0f0Gor0bllcPwcMcasgZq6EsZtao-GV1tYN-Rt_]--></g></svg>

## Key differences

| Feature                             | `<API_RENDER>`              | `<API_FRAGMENT_RENDER>`                |
| ----------------------------------- | --------------------------- | -------------------------------------- |
| Cache                               | Optional                    | None (runtime-forced `nocache = true`) |
| Client auth handling                | No                          | Yes (forms/tokens)                     |
| Upstream auth (Server→API)          | Yes (JWT/Basic/Cookies)     | Yes (JWT/Basic/Cookies)                |
| Query param filtering (`querykeys`) | Yes                         | Yes                                    |
| Request body mapping                | Yes                         | Yes                                    |
| Transform via `inline`/`template`   | Yes                         | Yes                                    |
| HTMX response headers               | No                          | Yes via `response { ... }`             |
| `setcookie` back to client          | No (unused in current code) | Yes (200 only)                         |

---

## Key fields (reference tables)

### `<API_RENDER>`

| Property            | Description                                                                                                                   |                       |
| ------------------- | ----------------------------------------------------------------------------------------------------------------------------- | --------------------- |
| endpoint            | API URL.                                                                                                                      |                       |
| method              | HTTP method (default GET).                                                                                                    |                       |
| cache               | Enable caching for a duration (e.g., `60s`, `5m`, `1h`). *(If caching is controlled at a parent level, document that there.)* |                       |
| nocache             | Force dynamic rendering, overriding cache. *(If controlled at a parent level, document precedence there.)*                    |                       |
| querykeys           | Allowlist of client query params to forward (default: `id`, `name`, `order`).                                                 |                       |
| queryparams         | Extra query params to append to the outgoing request.                                                                         |                       |
| headers             | Upstream request headers to send to the API.                                                                                  |                       |
| body                | String with `$key` placeholders replaced from query/form/json.                                                                |                       |
| inline / template   | Template source (inline block or file path).                                                                                  |                       |
| values              | Key-value pairs merged into the template context **root** (use `{{ .key }}`).                                                 |                       |
| username / password | Basic Auth credentials for upstream.                                                                                          |                       |
| jwtsecret           | Secret for generating a JWT for `Authorization`.                                                                              |                       |
| jwtclaims           | Claims map; `exp` is seconds offset from now.                                                                                 |                       |
| debug               | Adds debug comments.                                                                                                          |                       |
| debugpanel          | Enables the front-end error panel (non-LIVE mode and global flag on).                                                         |                       |
| enclose             | Wrap final HTML with `before                                                                                                  | after` (see Enclose). |

### `<API_FRAGMENT_RENDER>`

| Property            | Description                                                                                                       |                       |
| ------------------- | ----------------------------------------------------------------------------------------------------------------- | --------------------- |
| route               | Fragment route (URL segment).                                                                                     |                       |
| title               | Optional fragment title.                                                                                          |                       |
| section             | Logical grouping section.                                                                                         |                       |
| index               | Sort key for menus.                                                                                               |                       |
| endpoint            | API URL.                                                                                                          |                       |
| method              | HTTP method (default GET).                                                                                        |                       |
| nocache             | Always dynamic. **Runtime-forced true.**                                                                          |                       |
| querykeys           | Allowlist of client query params to forward (default: `id`, `name`, `order`).                                     |                       |
| queryparams         | Extra query params for the outgoing request.                                                                      |                       |
| request.headers     | Headers to send to the upstream API. *(If you also need non-HTMX response headers, add a separate config block.)* |                       |
| response { ... }    | HTMX response header block (see table below).                                                                     |                       |
| inline / template   | Template source (inline block or file path).                                                                      |                       |
| values              | Key-value pairs merged into the template context **root** (use `{{ .key }}`).                                     |                       |
| username / password | Basic Auth credentials for upstream.                                                                              |                       |
| jwtsecret           | Secret for generating a JWT (overrides cookie token).                                                             |                       |
| jwtclaims           | Claims map; `exp` is seconds offset.                                                                              |                       |
| setcookie           | Template that becomes `Set-Cookie` when `.Status == 200`.                                                         |                       |
| debug               | Adds debug comments.                                                                                              |                       |
| debugpanel          | Enables front-end error panel (non-LIVE mode and global flag on).                                                 |                       |
| enclose             | Wrap final HTML with `before                                                                                      | after` (see Enclose). |

### Enclose helper

| Property  | Description                                                                                                     |                            |         |
| --------- | --------------------------------------------------------------------------------------------------------------- | -------------------------- | ------- |
| enclose   | Enclosing HTML split by `                                                                                       | `, e.g. `<div class="box"> | </div>` |
| trimspace | *(Not currently documented as supported.)* If you add it later, define exact behavior.                          |                            |         |
| value     | *(Not currently documented as supported.)* If you add it later, define how it interacts with `inline/template`. |                            |         |

---

## Template context

| Property         | Description                                                                                 |
| ---------------- | ------------------------------------------------------------------------------------------- |
| `Data`           | Parsed API response (JSON object/array → map/list; XML may fall back; plain text → string). |
| `Status`         | Upstream HTTP status code.                                                                  |
| `values { ... }` | Merged into the template context **root** (use `{{ .key }}`).                               |


---

## Config variables

You can set variables and reuse them.

| Property        | Description                       |
| --------------- | --------------------------------- |
| `$NAME = value` | Defines a variable at file scope. |
| `{{VAR:NAME}}`  | Expands to the variable's value.  |

Example:

```properties
$API_URL = http://localhost:3000
endpoint = {{VAR:API_URL}}/rpc/login_user
```

Or you can use environment variables like this:

| Property        | Description                       |
| --------------- | --------------------------------- |
| `{{ENV:NAME}}`  | Expands to the environment variable's value.  |

Example:

```properties
endpoint = {{ENV:API_URL}}/rpc/login_user
```

**Note:** If variables are shared across imports/files, document precedence (nearest scope wins vs global) in one place.

---

## Mapping the request (query, form, body)

| Source       | Included in fragments        | Included in API_RENDER       | Notes                                                                 |
| ------------ | ---------------------------- | ---------------------------- | --------------------------------------------------------------------- |
| Query params | Yes, filtered by `querykeys` | Yes, filtered by `querykeys` | Default allowlist: `id`, `name`, `order`; empty list → forward none.  |
| Form data    | Yes                          | Yes                          | Flattened: single → string; multi → list.                             |
| JSON body    | Yes                          | Yes                          | Merged; on key collision, JSON key is also available as `body_<key>`. |

**Placeholders in `body`:** `$key` tokens are replaced from the merged data.

* Uses a word-boundary regex around key names. Stick to `[A-Za-z0-9_]+` in placeholder keys.
* Multi-value form fields are stringified. *(If you add joiners later, document the syntax.)*

---

## Authentication behavior

Observed precedence for the upstream `Authorization` header:

1. If `jwtsecret` is set, a new JWT is generated from `jwtclaims` and used.
2. Else, if the incoming client request has a `token` cookie, use `Bearer <token>`.
3. Else, if `username`/`password` are set, use Basic Auth.
4. Else, no auth header.

`jwtclaims.exp` is treated as **seconds offset from now**. If missing or invalid, defaults to now + 1h.

---

## Caching & static

* `cache = <duration>` enables caching for `<API_RENDER>`.
* `nocache = true` forces dynamic rendering.
* `<API_FRAGMENT_RENDER>` forces `nocache = true` at runtime.
* `static = <path>` writes static files during `hyperbricks static`.

**TO-DO:** Define precedence between parent composite cache settings and child settings.

---

## HTMX response headers

Configure inside `response { ... }` on fragments.

| Key                     | HTMX Header             | Description                              |
| ----------------------- | ----------------------- | ---------------------------------------- |
| hx_location             | HX-Location             | Client-side redirect without full reload |
| hx_push_url             | HX-Push-Url             | Push a new URL into history              |
| hx_redirect             | HX-Redirect             | Redirect to new location                 |
| hx_refresh              | HX-Refresh              | Full page refresh                        |
| hx_replace_url          | HX-Replace-Url          | Replace current URL in the location bar  |
| hx_reswap               | HX-Reswap               | Swap behavior                            |
| hx_retarget             | HX-Retarget             | Selector for update target               |
| hx_reselect             | HX-Reselect             | Selector to pick part of the response    |
| hx_trigger              | HX-Trigger              | Trigger client events                    |
| hx_trigger_after_settle | HX-Trigger-After-Settle | Trigger events after settle              |
| hx_trigger_after_swap   | HX-Trigger-After-Swap   | Trigger events after swap                |

---

## Security notes

* New cookie jar per outgoing request; shared transport for pooling. Prevents cookie leakage between users.
* If a `token` cookie exists on the client request, it becomes `Authorization: Bearer <token>` to the upstream unless `jwtsecret` overrides.
* `setcookie` runs only when `.Status == 200` on fragments. Prefer `HttpOnly; Secure; SameSite=Lax; Path=/` and set expiry/Max-Age.
* `querykeys` allowlist prevents accidental forwarding of sensitive client params.
* Sanitize any dynamic values you reflect into headers or cookies.

---

## Debug & errors

* `debug = true` adds a HTML comment with the upstream payload (in non-LIVE mode).
* `debugpanel = true` injects a front-end error panel when global `Development.FrontendErrors` is enabled (and not LIVE).
* Errors are surfaced as HTML comments and collected for logs.
* **TO-DO:** Fragment debug string says `Debug in <API_RENDER>...` — adjust label.


---

## Known limitations & open items

* XML decoding often needs struct bindings; generic map decoding may fail and fall back to plain text. *(Document what “fallback” looks like for templates.)*
* Placeholder matching uses word boundaries; avoid dashes in `$key` names. 
* Fragment rendering assumes `Request` and `ResponseWriter` are present in context.
* `<API_RENDER>` exposes `setcookie` in the struct but does not send it.



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











### headers

**Description**  
HTTP response headers to include when serving this hypermedia


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia {
    headers {
        X-Frame-Options = DENY
        Content-Security-Policy = default-src 'self'
    }
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <body></body>
</html>
````











### cookies

**Description**  
HTTP cookies to include when serving this hypermedia


**Example**
````properties
hypermedia = <HYPERMEDIA>
hypermedia {
    cookies = [session=abc; Path=/; HttpOnly; Secure, prefs=dark; Path=/; Max-Age=31536000; SameSite=Lax]
}

````

**Expected Result**

````html
<!DOCTYPE html>
<html>
  <body></body>
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











