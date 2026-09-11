# API security integration fixture

This repository-owned module proves the credential boundary shared by
`api_render` and `api_fragment_render`. It is an integration fixture, not a
starter application. Every token is a fixed public test value.

The complete [cookie-boundaries research article](docs/Cookie_Boundaries_and_Upstream_Token_Forwarding.md)
explains the original problem, the browser/upstream distinction, and the implemented
decision in section 23. Its earlier sections are clearly marked as historical
analysis. See [API Render](../../docs/API_RENDER.md) for the current field contract.

The main scenario issues two browser cookies and then renders four API
components in one response:

| Component | Configuration | Expected upstream authorization |
| --- | --- | --- |
| User-private API | `forwardtoken: user_session` | `Bearer fixture-user-token` |
| Admin-private API | `forwardtoken: admin_session` | `Bearer fixture-admin-token` |
| Public API | `forwardtoken` omitted | none |
| Service API | configured `headers.Authorization` | `Bearer fixture-service-token` |

The cookie names and upstream JSON fields are deliberately different concepts.
The login response fields `.Data.user_token` and `.Data.admin_token` supply the
values. The structured `setcookies` entries choose the browser cookie names.
Later `forwardtoken` values name exactly which incoming browser cookie a
component may translate into a Bearer header.

## Automated test

From the repository root:

```sh
go test ./cmd/hyperbricks -run '^TestAPISecurityModuleCredentialAndCookieBoundaries$' -count=1
```

The test loads this package and its YAML/templates through the ordinary module
pipeline. It runs HyperBricks, the primary upstream, and a second redirect
target as local HTTP test servers. It covers:

- Two cookies routed to two matching private APIs during one composed render
- A nested public API receiving no browser credential because `forwardtoken` is omitted
- A public fragment receiving no browser credential with explicit `forwardtoken: ""`
- Explicit service authentication remaining component-local
- `api_fragment_render` using the same named-cookie contract
- Missing, empty, and syntactically invalid issued token values
- Atomic `setcookies`: one bad entry prevents every entry from being emitted
- Invalid upstream JSON and main response-template errors suppressing otherwise
  valid cookies
- Explicit deletion through `max_age: 0` after an upstream `204`
- Duplicate cookies with one carrier name rejecting before the upstream call
- A missing or differently named carrier producing no browser-derived header
- Incoming browser `Authorization` never acting as an implicit forwarding source
- Conflicting component authentication settings rejecting before the upstream call
- Raw Boolean and numeric `forwardtoken` values remaining type errors
- Cross-port redirects being rejected before the second origin receives a request
- Concurrent composed API calls overlapping at a controlled upstream barrier
- Browser request cancellation promptly cancelling both API component types
- Real renderer debug output retaining metadata while omitting credential, path,
  query, and header-value secrets

## Manual walkthrough

Build or select a HyperBricks binary from this checkout. Start the deterministic
fixture API in one terminal:

```sh
go run ./modules/api-security-test/tools/mock-api -port 8098 -redirect-port 8099
```

Start this module in another terminal:

```sh
HYPERBRICKS_API_SECURITY_UPSTREAM=http://127.0.0.1:8098 \
  hyperbricks start -m api-security-test --port 8105
```

To run the current source without installing a binary, use the same environment
variable with `go run ./cmd/hyperbricks start -m api-security-test --port 8105`.
Both commands run from the repository root. A previously installed release may
not yet contain this fixture's credential and structured-cookie contract.

Use a temporary curl cookie jar to perform login and the later composed request:

```sh
cookie_jar="$(mktemp)"
curl -i -X POST -c "$cookie_jar" http://127.0.0.1:8105/actions/login
curl -i -b "$cookie_jar" http://127.0.0.1:8105/dashboard
```

The API console reports only an authorization classification. It should show a
user session for `/private/user`, an admin session for `/private/admin`, no
authorization for `/public`, and service authorization for `/service`. It never
prints a received credential value.

The fragment component uses the same user carrier:

```sh
curl -i -b "$cookie_jar" http://127.0.0.1:8105/actions/private
```

The composed barrier route finishes only after both private upstream requests
have arrived, so one request also demonstrates real fanout instead of a serial
pair of calls:

```sh
curl -i -b "$cookie_jar" http://127.0.0.1:8105/concurrent-fanout
```

The two debug routes exercise the actual renderer diagnostics. Inspect the
HyperBricks terminal after these requests: it reports the method, origin,
header names, and response status, while omitting cookie values, authorization
values, API-key values, endpoint paths, and query strings.

```sh
curl -i -b "$cookie_jar" http://127.0.0.1:8105/debug/nested
curl -i -b "$cookie_jar" http://127.0.0.1:8105/debug/fragment
```

The mock API also leaves `/cancel/nested` and `/cancel/fragment` open until the
browser request is cancelled. The automated test starts each route, cancels its
incoming request after the upstream observes it, and requires both the nested
and fragment upstream contexts to finish within two seconds.

These two routes deliberately fail after the upstream call. The first returns a
`200 application/problem+json` body with trailing data; the second decodes a
valid response and then fails while executing the main fragment template. Both
responses report a render error and must contain no `Set-Cookie` header:

```sh
curl -i -X POST http://127.0.0.1:8105/actions/login-invalid-json
curl -i -X POST http://127.0.0.1:8105/actions/login-template-error
```

Logout receives upstream `204` and returns two explicit deletion cookies:

```sh
curl -i -X POST -b "$cookie_jar" -c "$cookie_jar" http://127.0.0.1:8105/actions/logout
curl -i -b "$cookie_jar" http://127.0.0.1:8105/dashboard
```

After logout, both private API calls are still made but contain no browser-derived
authorization. The explicitly authenticated service call remains authenticated.

A duplicate carrier is rejected before `/private/duplicate` is called:

```sh
curl -i -H 'Cookie: user_session=first; user_session=second' \
  http://127.0.0.1:8105/duplicate-cookie
```

The two redirect routes call an approved endpoint on port 8098 which attempts to
redirect to port 8099. Port is part of the origin, so HyperBricks rejects the
redirect and the target server must report no request:

```sh
curl -i -H 'Cookie: user_session=fixture-user-token' \
  http://127.0.0.1:8105/redirects/nested
curl -i -H 'Cookie: user_session=fixture-user-token' \
  http://127.0.0.1:8105/redirects/fragment
```

## Security decisions represented by the fixture

`forwardtoken` is a string. Omission and the empty string disable browser-token
forwarding. Literal YAML Boolean or numeric values are configuration errors;
`forwardtoken: false` is not a second spelling of omission. This keeps the final
schema one type and prevents weak decoding from converting unrelated values into
cookie names.

This is component-local validation. YAML preserves the source types for these
API security fields, and the API component validates them before weak decoding.
Structured cookie entries retain their validated booleans and integers through
the second decoding step. Other components keep their existing YAML conversion
rules.

Only one non-empty incoming cookie with the configured name is accepted. An
incoming browser `Authorization` header and differently named cookies are not
fallback sources. Duplicate cookies are ambiguous because the request header no
longer contains their original browser scopes, so the component fails before
making its upstream request.

Each component has one authentication owner. `forwardtoken` cannot be combined
with generated JWT, Basic Auth, or configured `Authorization`. Service
credentials remain explicit component configuration and do not depend on the
browser session.

Structured response cookies keep API response data in the cookie value rather
than treating it as raw header syntax. All configured entries are rendered and
validated before any `Set-Cookie` header is added. A missing field, an empty
issued credential, or an invalid value rejects the complete group. Empty values
are accepted only for an explicit deletion with `max_age: 0`; omitting `max_age`
creates a session cookie.

This fixture uses `secure: false` because its documented manual workflow runs
entirely over loopback HTTP. A deployed authentication cookie should use
`secure: true`, `http_only: true`, an explicit path, a suitable SameSite policy,
and normally no `domain` so it stays host-only. The fixture keeps `http_only`,
`path`, and SameSite explicit so their serialized behavior is still covered.

The credential-bearing upstream URLs are loopback HTTP URLs and are accepted
only because the package runs in development mode. Credential-bearing live
integrations must use HTTPS. Redirects must retain the exact scheme, hostname,
and effective port; changing any of those values requires a new explicitly
configured recipient rather than inherited redirect authority.

## Source layout

| Path | Responsibility |
| --- | --- |
| `package.hyperbricks.yaml` | Development-only module configuration |
| `hyperbricks/api-security.hyperbricks.yaml` | Positive and negative credential-boundary routes |
| `templates/` | API and action response templates |
| `tools/mock-api/` | Deterministic two-origin API for the manual walkthrough |
| [server_api_security_module_test.go](../../cmd/hyperbricks/server_api_security_module_test.go) | Automated end-to-end assertions |
| [auth.go](../../pkg/shared/apiutil/auth.go) / [transport.go](../../pkg/shared/apiutil/transport.go) | Shared credential selection and upstream transport policy |
| [api_response_cookies.go](../../pkg/composite/api_response_cookies.go) | Cookie value rendering and validation |
| [server_api_cookie_commit_test.go](../../cmd/hyperbricks/server_api_cookie_commit_test.go) | Cookie suppression after late rendering errors and response-owner conflicts |

For the complete repository regression suite, run `./tests.sh --with-docs` from
the repository root. This fixture uses no plugins. The targeted test at the top
of this README is sufficient to rerun its own security scenarios independently.
