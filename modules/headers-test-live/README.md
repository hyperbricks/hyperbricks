# headers-test-live

Live-mode regression fixture for HyperBricks response headers, cookies, and
cached page delivery. This is a test module, not an application starter.

Its [YAML source](hyperbricks/hello-world.hyperbricks.yaml) defines an
`index.html` page displaying `HELLO WORLD!` and a `help.html` page displaying
`HELP PAGE`. Both set `X-Test: headers-test-live`, a `default-src 'self'` Content
Security Policy, and fixed `session` and `prefs` test cookies.

The [package configuration](package.hyperbricks.yaml) selects live mode with a
30-second cache. [scripts/test_headers_module.sh](../../scripts/test_headers_module.sh)
checks the content, headers, cookies, clean-URL aliases, and unknown-route 404s.
It also requires `X-Hyperbricks-Rendered-At` and
`X-Hyperbricks-Cache-Expires-At`, then repeats the page request to verify that
the rendered timestamp is unchanged while the cached response is reused.

Compare [headers-test](../headers-test/README.md) for the development-mode
counterpart, where those live-cache timestamp headers must be absent.
