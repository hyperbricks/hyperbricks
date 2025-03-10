### API Serverside Render (`<API_RENDER>` & `<API_FRAGMENT_RENDER>`)**

### `<API_FRAGMENT_RENDER>`

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

`<API_FRAGMENT_RENDER>` Can handle Client auth requests based on login forms and tokens that will be passed thrue bi-directional.
| `Client->Server` | `<API_RENDER>` | `<API_FRAGMENT_RENDER>` |
|----------------------|-----------------------------|-----------------------------|
| **Client->Server: JWT Authentication (`jwtsecret`)** | ❌ No | ✅ Yes |
| **Client->Server: Session-Based Auth (Cookies)** | ❌ No | ✅ Yes |
| **Client->Server: Basic Auth username and password** |❌ No  | ✅ Yes |
| **Client->Server: Generates JWT with Claims (`jwtclaims`)** | ❌ No | ✅ Yes |
| **Client->Server: Body and formdata mapping** | ✅ Yes (for public API, non-cached) | ✅ Yes |

### **Server->API Interaction**
Both components can aply authentication on API requests. So for example a Weather Service that requires a 
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

### Rendering Order and Property Rules