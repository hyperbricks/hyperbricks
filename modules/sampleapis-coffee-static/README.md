# SampleAPIs Coffee Static Demo

This module demonstrates the static snapshot path with a real nested `api_render`.

The route in `hyperbricks/coffee-static.hyperbricks.yaml` fetches `https://api.sampleapis.com/coffee/hot`, renders the JSON through `templates/coffee-cards.html`, and writes the final HTML to `rendered/index.html` when static rendering runs.

Static rendering this module requires internet access. The generated HTML can change when SampleAPIs changes its coffee data.

## Run It Dynamically

```bash
go run ./cmd/hyperbricks start -m sampleapis-coffee-static
```

Open `http://localhost:8080/index.html`.

## Render Static HTML

```bash
go run ./cmd/hyperbricks static -m sampleapis-coffee-static --force
```

Expected output:

- `modules/sampleapis-coffee-static/rendered/index.html`
- `modules/sampleapis-coffee-static/rendered/static/coffee.css`

## Render And Then Serve The Static Result

```bash
go run ./cmd/hyperbricks static -m sampleapis-coffee-static --serve
```

`--serve` rebuilds the snapshot, which calls the coffee API during rendering,
and then serves `modules/sampleapis-coffee-static/rendered`.

To serve an existing snapshot without fetching the API again, use a standalone
file server instead:

```bash
python3 -m http.server 8080 --directory modules/sampleapis-coffee-static/rendered
```

## What To Copy Into Another Module

- Put API-rendered page routes in `hyperbricks/*.hyperbricks.yaml`.
- Put HTML templates in `templates/`.
- Put CSS, images, or JavaScript in `static/`.
- Add explicit snapshot targets under `hyperbricks.static.routes` in `package.hyperbricks.yaml` when you want clear output filenames.

The key pattern is:

```yaml
coffee_menu:
  - type: api_render
  - endpoint: https://api.sampleapis.com/coffee/hot
  - method: GET
  - template:
      file: coffee-cards.html
```
