# Unpoly guard demo

A protected page and fragment served by HyperBricks, with Unpoly 3.14.3 handling fragment requests. No HTMX or core changes are required.

## Build and run

From the project root, with Go 1.26.1 or newer and a compatible installed published HyperBricks release:

```sh
hyperbricks plugin build guard-auth@1.0.0 --module unpoly-guard-demo
hyperbricks start -m unpoly-guard-demo --non-interactive
```

Leave `HYPERBRICKS_LOCAL_PATH` unset for an installed published release. Building plugin source does not require a local HyperBricks checkout.

For development against the local HyperBricks source, run from the checkout root:

```sh
HYPERBRICKS_LOCAL_PATH="$PWD" go run ./cmd/hyperbricks plugin build guard-auth@1.0.0 --module unpoly-guard-demo
go run ./cmd/hyperbricks start -m unpoly-guard-demo --non-interactive
```

`HYPERBRICKS_LOCAL_PATH` is a development-only override. If the local CLI was installed with `go install ./cmd/hyperbricks`, use the same override with `hyperbricks plugin build`. Native plugins and their host must use matching source and toolchains. See [plugin build modes](../../docs/PLUGINS.md#local-runtime-development).

Open [localhost:8132](http://localhost:8132/). Unpoly is included locally; native esbuild builds the application JavaScript and CSS. No separate authorization server is needed.

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
