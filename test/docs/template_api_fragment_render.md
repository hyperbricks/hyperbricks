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