# Hello World

The smallest HyperBricks starter: one YAML route that returns a short page.

From the HyperBricks repository root, run:

```sh
hyperbricks start -m hello-world
```

Open [http://127.0.0.1:8080/](http://127.0.0.1:8080/). The source is in
[`hyperbricks/hello-world.hyperbricks.yaml`](hyperbricks/hello-world.hyperbricks.yaml).
Edit the text there and refresh while development watching is enabled.

The page runs without a developer login. Its dashboard is disabled by default.
If you enable the developer interface in `package.hyperbricks.yaml`, set
credentials before startup; there is no built-in account:

```sh
export HB_DEVELOPER_USER=developer
export HB_DEVELOPER_PASSWORD='choose-a-long-password'
```

To create your own copy in another HyperBricks project, use
`hyperbricks init-starter get hello-world -m my-site`.
