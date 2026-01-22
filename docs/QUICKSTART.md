# Quickstart

## 1) Install

Requires Go **1.23.2+**.

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
```

---

## 2) Create a module

From your project root (the folder that will contain `modules/`):

```bash
hyperbricks init -m someproject
```

You’ll get:

```
rootdir/
└── modules/
    └── someproject/
        ├── hyperbricks/
        ├── rendered/
        ├── resources/
        ├── static/
        ├── templates/
        └── package.hyperbricks
```

> Always run the CLI from `rootdir/` (the parent of `modules/`).

---

## 3) Add your first page route (`<HYPERMEDIA>`)

Create: `modules/someproject/hyperbricks/index.hyperbricks`

```hyperbricks
docs = <HYPERMEDIA>
docs.route = index
docs.title = HyperBricks | Quickstart
docs.htmltag = <html class="bg-gray-950 text-gray-200">
docs.bodytag = <body class="p-8">|</body>

docs.10 = <HTML>
docs.10.value = <<[
  <h1 class="text-2xl font-bold mb-4">Hello HyperBricks</h1>

  <button
    class="px-3 py-2 rounded bg-white text-black"
    hx-get="/hello_fragment"
    hx-target="#target"
    hx-swap="innerHTML"
  >
    Load fragment
  </button>

  <div id="target" class="mt-4 p-4 border border-white/20 rounded">
    (fragment loads here)
  </div>

  <script src="https://unpkg.com/htmx.org@2.0.4"></script>
]>>
```

---

## 4) Add your first fragment route (`<FRAGMENT>`)

Create: `modules/someproject/hyperbricks/hello_fragment.hyperbricks`

```hyperbricks
hello = <FRAGMENT>
hello.route = hello_fragment

# Optional: HTMX response headers
hello.response {
  hx_trigger = helloLoaded
  hx_reswap = innerHTML
}

hello.10 = <HTML>
hello.10.value = <<[
  <div>
    <h2 class="text-xl font-semibold">Hi from a fragment</h2>
    <p>This HTML was returned without a full page reload.</p>
  </div>
]>>
```

---

## 5) Run the dev server

From `rootdir/`:

```bash
hyperbricks start -m someproject
```

Open:

* `http://localhost:8080/index`

Click **Load fragment** — it should call `/hello_fragment` and swap into `#target`.

---

## 6) (Optional) Use templates from `templates/`

If you want a reusable head/body structure, create a template file:

`modules/someproject/templates/head.html`:

```html
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>{{ .title }}</title>
<script src="https://unpkg.com/htmx.org@2.0.4"></script>
```

Then reference it from `<HYPERMEDIA>`:

```hyperbricks
docs.head.100 = <TEMPLATE>
docs.head.100.template = {{TEMPLATE:head.html}}
docs.head.100.values {
  title = HyperBricks | Quickstart
}
```

(You can keep page content in `docs.10`, `docs.20`, etc., as usual.)

---

## 7) Render static output

```bash
hyperbricks static -m someproject
```

Static output is written to `modules/someproject/rendered/` (by default). Serve that folder however you like.