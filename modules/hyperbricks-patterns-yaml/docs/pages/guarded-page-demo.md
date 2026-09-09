# Guarded Page Demo

## Summary

This pattern shows a simple protected-page flow:

- landing page at `/guarded-demo`
- public login page
- native route guard on the protected page
- real forbidden page route
- HTMX login/logout flow handled by a small plugin endpoint

## Routes

- `/guarded-demo`
- `/guarded-demo/login`
- `/guarded-demo/secret`
- `/guarded-demo/forbidden`

Auth endpoints:

- `/guarded-demo/auth/login`
- `/guarded-demo/auth/logout`
- `/guarded-demo/auth/authorize`

## Files

- Config: `hyperbricks/30-guarded-page-demo.hyperbricks.yaml`
- Plugin: `plugins/guarded-demo-auth/1.0.0/guarded_demo_auth_plugin.go`
- Templates: `templates/patterns/guarded-shell.html` `templates/patterns/guarded-login.html` `templates/patterns/guarded-secret.html` `templates/patterns/guarded-forbidden.html`

## Demo behavior

- `/guarded-demo` is the main landing page for the pattern.
- `/guarded-demo/login` is just an alias route to the same login panel.
- Successful login sets `guarded_demo_session` and redirects to `/guarded-demo/secret`.
- A blocked demo user redirects to `/guarded-demo/forbidden`.
- Logout clears the same cookie and redirects back to `/guarded-demo`.

Demo credentials:

- `demo / open-sesame` -> secret page
- `blocked / open-sesame` -> forbidden page

## How it works

Think of the flow like this:

- the user opens a public login page
- the login request sets a browser cookie
- the protected page checks that cookie before it renders
- missing or invalid credentials send the user to `/guarded-demo/login`
- valid credentials without access send the user to `/guarded-demo/forbidden`

## Guard contract

The important detail is the authorize contract for the native route guard.

- The login endpoint sets the browser cookie.
- The guarded page declares `auth.cookie = guarded_demo_session`.
- HyperBricks resolves that cookie on the incoming request and forwards the value to `authorize.endpoint` as `Authorization: Bearer <token>`.
- The authorize endpoint must validate that bearer token. It should not rely only on reading the cookie again.

The guard explicitly configures each denial response. Ordinary requests receive `303` and `Location`. A variant matching `HX-Request: "true"` uses `401` or `403` with `HX-Redirect`. The first matching variant replaces the default; HyperBricks does not convert the redirect automatically. The plugin's own HTMX login/logout handling remains application logic.

## Pattern rule

Keep responsibilities separate:

- page shell decides who may render the protected route
- auth plugin handles login/logout/authorize request mechanics
- forbidden is a real route, not just an inline message

That matches the same ownership pattern used by Composer: page shells own route access, while auth endpoints own login/logout/authorize mechanics.
