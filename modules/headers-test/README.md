# headers-test

Development-mode regression fixture for HyperBricks response headers, cookies,
and clean-URL routing. This is a test module, not an application starter.

Its [YAML source](hyperbricks/hello-world.hyperbricks.yaml) defines two small
pages: `index.html` displays `HELLO WORLD!`, and `help.html` displays `HELP PAGE`.
Both pages set:

- `X-Test: headers-test-dev` and a `default-src 'self'` Content Security Policy.
- Fixed test cookies named `session` and `prefs`, with HttpOnly or SameSite
  attributes; these are fixture values, not an authentication implementation.

[scripts/test_headers_module.sh](../../scripts/test_headers_module.sh) checks
the rendered content, custom header, both cookies, `/index` and `/help` aliases,
and a 404 for an unknown route. It also checks that live-cache timestamp headers
are absent in development mode.

The paired [headers-test-live](../headers-test-live/README.md) fixture covers
the same responses with live-mode caching enabled.
