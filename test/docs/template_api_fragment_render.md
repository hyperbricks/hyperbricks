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
