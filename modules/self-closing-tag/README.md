# Self-closing tag fixture

A small manual verification module for image rendering and the
`hyperbricks.server.self_closing_tags` setting.

The index page renders the bundled cat image at 100 pixels wide, with lazy
loading, an ID, and a title. HyperBricks writes the processed image to the
module's static image directory and generates its `<img>` markup. The package
enables self-closing tags so the response source can be inspected for the
trailing `/>`; switching the setting off allows comparison with `>`.

Run from the repository root:

```sh
go run ./cmd/hyperbricks start -m self-closing-tag --port 8080
```

Open <http://localhost:8080/> and inspect the response source, not just the
browser's rendered DOM. No plugin or external API is required. This is a
focused verification fixture, not an application starter.
