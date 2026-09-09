# Unpoly guard demo

A protected page and fragment served by HyperBricks, with Unpoly 3.14.3 handling fragment requests. No HTMX or core changes are required.

## Build and run

From the HyperBricks repository root, using Go 1.26.1 or newer:

```sh
go build -o ./bin/hyperbricks-unpoly-guard ./cmd/hyperbricks
HYPERBRICKS_LOCAL_PATH="$PWD" ./bin/hyperbricks-unpoly-guard \
  plugin build guard-auth@1.0.0 --module unpoly-guard-demo
./bin/hyperbricks-unpoly-guard start -m unpoly-guard-demo --non-interactive
```

Build the binary and plugin from the same checkout and Go toolchain. Open http://localhost:8132/. Unpoly is included locally; native esbuild builds the application JavaScript and CSS. No separate authorization server is needed.

## Try it

1. Without signing in, click **Load protected fragment**. The server returns 401; the Unpoly integration opens the complete **Sign in** page.
2. Sign in with `member` / `open-sesame`. Load the fragment again: only the panel updates. The **Protected page** link also opens successfully.
3. Click **End session without leaving**, then load the fragment. The server rejects the revoked session and the browser opens Sign in again.
4. Sign in with `blocked` / `open-sesame`. Loading the fragment opens **Access denied** because the valid account is not authorized.
5. Open `/private` directly without a session: the server sends an ordinary 303 redirect to `/login`, independently of Unpoly.

The note field demonstrates DOM preservation during successful panel updates. It is saved locally in this browser and restored after navigation/reload.

## Where the behavior lives

| File | Responsibility |
| --- | --- |
| `hyperbricks/app.hyperbricks.yaml` | Pages, protected fragment, native guards and denial-response variants |
| `resources/js/app.js` | Unpoly requests and panel rendering; full navigation on 401/403 |
| `plugins/guard-auth/1.0.0/auth.go` | Demo credentials, random server-side sessions, authorization and logout |
| `templates/` | External page templates |
| `resources/css/app.css` | Styling bundled with native esbuild |
| `package.hyperbricks.yaml` | Module settings and enabled plugin |

The application adds `X-Demo-Client: unpoly` to fragment requests. This is our explicit marker, not a built-in Unpoly header. The guard matches it exactly to choose 401/403 instead of a normal 303 redirect. It never grants access.

`require.authenticated` checks for a token; `authorize.endpoint: /auth/authorize` validates that token and checks the account's access. Both `/private` and `/fragments/private` have a guard. Denied requests do not render their content. Guard responses use `Cache-Control: no-store`; Unpoly requests also disable cache.

The JavaScript uses `up.request()` to distinguish denial statuses before passing a successful `up.Response` to `up.render()`. It uses fixed local login/forbidden URLs. It does not treat `X-Up-Location` as an equivalent of `HX-Redirect`.

## Demo boundaries

This is a local demonstration, not a production authentication service. Account names/passwords are deliberately public. Sessions use random 256-bit tokens, expire after 15 minutes, are capped at 128, and disappear on server restart. A mutex protects the session map; authorization returns a request-local snapshot. Logout deletes the session server-side and expires its cookie.

Cookies are HttpOnly and SameSite=Lax, and Secure when the request uses TLS. Login/logout require POST and a custom action header; no CORS access is enabled. Use a production identity service, deployment-appropriate cookie/TLS settings, and appropriate abuse controls in a real application. Ordinary HTTP requests cannot bypass the guard by omitting or forging the demo client marker.

## Credits

[HyperBricks](https://hyperbricks.org/) · [HyperBricks repository](https://github.com/hyperbricks/hyperbricks) · [Unpoly](https://unpoly.com/) · [Unpoly repository](https://github.com/unpoly/unpoly). Unpoly 3.14.3 is copied from the existing todo-demo-unpoly local distribution; its MIT license is in `static/vendor/LICENSE.txt`.
