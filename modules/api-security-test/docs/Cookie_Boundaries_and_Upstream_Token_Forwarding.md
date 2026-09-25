# API credential boundaries and regression coverage

This note explains the security decisions exercised by the
[`api-security-test` fixture](../README.md). The canonical configuration contract
is in [API Render](../../../docs/API_RENDER.md); the fixture demonstrates that
contract using local upstream servers and fixed public test credentials.

## Browser cookies and upstream credentials

The browser sends cookies to HyperBricks according to their browser scope.
An API component creates a separate server-to-server request. Turning a cookie
value into an upstream Bearer header is an explicit credential delegation:

```text
Browser cookie: user_session=...
    → component configured with forwardtoken: user_session
    → Authorization: Bearer ... sent to its configured upstream
```

Cookie attributes such as `Secure`, `HttpOnly` and `SameSite` protect aspects of
the browser interaction. They do not authorize another service to receive a
copied value. The application must choose an appropriate cookie and upstream
recipient for each component. Adding a public API component with `forwardtoken`
omitted does not automatically give it the browser's credentials.

## Enforced boundaries

- **Explicit cookie selection.** `forwardtoken` accepts an incoming cookie name
  as a string. Omission or `""` disables browser-token forwarding. A missing or
  empty selected cookie adds no Bearer header; duplicate selected cookies or
  malformed cookie headers are rejected before the upstream call. The incoming
  browser `Authorization` header is never an implicit fallback.
- **Unambiguous built-in authentication.** Choose at most one of `forwardtoken`,
  configured `headers.Authorization`, generated JWT, or complete Basic Auth.
  Conflicting sources, incomplete Basic credentials, JWT claims without a signing
  secret, and credentials embedded in endpoint URLs are rejected.
- **Secure transport for configured credentials.** Credential-bearing API
  requests require HTTPS. Nonempty configured bodies and custom header values
  other than `Accept` and `Content-Type` also trigger this requirement. HTTP is
  allowed for those requests only to literal loopback IPs in development/debug
  mode; a hostname such as `localhost` does not qualify.
- **Origin-bound redirects.** Redirects must retain the initial scheme,
  hostname and effective port, and cannot introduce URL user information.
  The permitted boundary is the origin: a same-origin redirect can change the
  path. Redirect loops are bounded, and requests inherit browser-request
  cancellation.
- **No upstream cookie jar.** Upstream `Set-Cookie` headers are not replayed
  during redirects, shared with sibling components, or automatically returned
  to the browser.
- **Validated response cookies.** A fragment's configured `setcookies` values
  are validated as a group after a successful upstream response and successful
  decoding/template rendering. If an entry fails, none of that group's cookies
  is issued. The HTTP runtime commits staged API cookies only after successful
  rendering; late render errors and plugin-owned responses suppress them.
  Explicit deletion uses `max_age: 0`.
- **Limited API diagnostics.** API component debug output reports metadata such
  as method, origin, header names and response status. It omits credential/header
  values, payloads, and endpoint paths and query strings.

These rules are shared by `api_render` and `api_fragment_render`. Authentication
selection is implemented in [auth.go](../../../pkg/shared/apiutil/auth.go),
redirect policy in [transport.go](../../../pkg/shared/apiutil/transport.go), and
cookie validation in
[api_response_cookies.go](../../../pkg/composite/api_response_cookies.go).

## Application responsibilities

Explicit forwarding identifies the credential recipient; it does not validate
the token's issuer, signature, audience, expiry, scope or revocation. The
receiving service must perform its own authentication and authorization. Use a
[route guard](../../../docs/ROUTE_GUARD.md) when access to a HyperBricks route
requires authentication; a missing forwarded cookie alone does not deny access.

Configure browser-cookie scope, `Secure`, `HttpOnly`, SameSite and CSRF protection
for the application. The fixture deliberately uses loopback HTTP and public test
tokens; its `secure: false` cookies are not production defaults to copy.

Keep secrets out of endpoint paths and query strings: the HTTPS credential
classifier does not inspect those values. Custom headers are explicitly copied
configuration and are not part of the four-source authentication conflict check.
Review each configured header and endpoint as part of the application's trust
policy. This policy does not provide a general SSRF defense, and route-guard
authorization calls and custom plugins have separate behavior.

Personalized responses also need an explicit cache policy. Set `nocache: true`
on the owning page or fragment when personalized data must be rendered per
request. This does not configure browsers, proxies or CDNs. See
[Internal Caching and HTTP Caching](../../../docs/LIVE_MODE_HTTP.md#internal-caching-and-http-caching).

## Why forwarding is opt-in

An earlier implementation automatically copied a nonempty browser cookie named
`token` into API requests. Composing an authenticated page with additional API
components could therefore send the credential to unintended recipients.
The current implementation removes that implicit behavior. During migration,
add `forwardtoken: token` only to components whose upstream is an intended
recipient; leave it omitted for public or explicitly service-authenticated calls.
See [Migration](../../../docs/MIGRATION.md). The original investigation is
preserved in this file's Git history.

## Regression coverage

From the repository root, run the fixture's integration test:

```sh
go test ./cmd/hyperbricks -run '^TestAPISecurityModuleCredentialAndCookieBoundaries$' -count=1
```

It loads the actual module YAML and templates and exercises named user/admin
cookies, a public API without forwarding, and component-local service
authentication. It also checks invalid or ambiguous credentials, cookie issuance
and deletion, cross-port redirect rejection, concurrent API calls, cancellation,
and debug redaction. The [fixture README](../README.md) provides the manual
walkthrough and the complete scenario list.

[server_api_cookie_commit_test.go](../../../cmd/hyperbricks/server_api_cookie_commit_test.go)
covers cookie suppression after late render errors and response-owner conflicts.
Focused unit tests beside the authentication, transport and cookie code cover
their validation rules. These tests verify the runtime boundaries with controlled
upstreams; deployments still require review of their own credentials, endpoints,
authorization and cache configuration.
