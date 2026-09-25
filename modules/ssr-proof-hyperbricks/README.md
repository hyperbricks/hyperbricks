# SSR proof fixture

A small benchmark and regression fixture for nested server-side rendering.
It supplies a repeatable workload, not a starter application or published
performance result. It uses inline templates and trees without plugins.

- `/` renders a landing page with nested sections and feature cards.
- The `rid` query parameter appears in three places to check request isolation.
- `/healthz` returns `ok`; `/start` returns a small text response.

The landing page and health route disable response caching. The landing page
also sends `Cache-Control: no-store`, even though both package profiles configure
a 30-second live-cache default for other routes.

`package.hyperbricks.yaml` uses live mode, port 8093, `GOMAXPROCS=4`,
30-second server timeouts and a high rate limit. `package.raw.hyperbricks.yaml`
keeps the same workload but disables server timeouts and rate limiting.

Repository benchmarks compare render pipelines and allocations using this YAML;
regression tests check equivalent output and isolation between concurrent requests.
