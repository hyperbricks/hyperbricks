# API response status fixture

This small module exercises `response_status` for a nested `api_render`, a
route-owning `api_fragment_render`, and an embedded API fragment. It uses plain
text responses so the HTTP contract is visible without browser code. It is an
integration fixture, not a starter application.

Run these commands from the repository root in separate terminals, using the
current checkout so the new feature is available:

```sh
go run ./modules/api-response-status-test/tools/mock-api -port 8114
```

```sh
HYPERBRICKS_STATUS_UPSTREAM=http://127.0.0.1:8114 \
  go run ./cmd/hyperbricks start -m api-response-status-test --port 8115
```

The default API URL is the same loopback address, so the environment variable is
only needed to select a different port. No external API or credentials are used.

Inspect the nested page and API fragment directly:

```sh
curl -i 'http://127.0.0.1:8115/catalog/detail?id=missing'
curl -i 'http://127.0.0.1:8115/fragments/detail?id=missing'
curl -i 'http://127.0.0.1:8115/catalog/embedded-fragment?id=missing'
```

Each returns HTTP 404 and `Cache-Control: no-store`, retaining its rendered
body. The complete 20-check HTTP walkthrough passed against the implementation
on 4 October 2026; see the
[workdocument](../../logs/20261004-1122-api-response-status-workdocument.md#completed-focused-verification)
for its scope and the separate regression-test requirements.

| Query `id` | Actual API response | Configured page/fragment result |
| --- | ---: | ---: |
| `existing` | 200 | 200 |
| `missing` | 404 | 404 |
| `unavailable` | 503 | 503 |
| `conflict` | 409 | 200 because the map says `ignore` |
| `forbidden` | 403 | 502 because the required API has no mapping for 403 |
| `invalid-json` | 404 with invalid JSON | 502 because decoding failed; the 404 map does not apply |

`/fragments/optional?id=missing` demonstrates the original behavior without a
policy: the body reports upstream 404 while the browser receives HTTP 200.
The configured nested page retains the existing `api_render` non-2xx diagnostic;
the fragment retains its existing behavior of rendering decoded non-2xx data
without a status-only error. Mapping and `ignore` do not suppress diagnostics.

The embedded fragment explicitly declares its policy at the mount. Direct API
inheritance strips the base policy; inheriting a whole configured page preserves
policies already mounted on its nested children. Every dynamic error is
no-store and discards staged API cookies. There are no cookies in this fixture.

See [API Render](../../docs/API_RENDER.md#browser-status-from-api-results) for
the complete contract and [HTTP Responses](../../docs/HTTP_RESPONSES.md#dynamic-api-status-precedence)
for final response ownership and precedence.
