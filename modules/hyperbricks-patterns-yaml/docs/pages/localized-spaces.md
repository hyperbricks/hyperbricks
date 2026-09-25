# Localized pages with Spaces

## What this pattern shows

Use one route-less `hypermedia` source for each page shape, then create one Space
instance per language and page. The sources own the shared template, navigation
structure, default copy, and editable field declarations. Each instance owns its
route, page title, document language, metadata, and translated values.

Open the [English home](/localized-spaces) or [German home](/localized-spaces/de).
Both link to their localized About page and back to the matching page in the other
language. Every localized page also links directly to the [pattern index](/index).

| Source | English Space and route | German Space and route |
| --- | --- | --- |
| `localized_home_source` | `localized_home_en` at `/localized-spaces` | `localized_home_de` at `/localized-spaces/de` |
| `localized_about_source` | `localized_about_en` at `/localized-spaces/about` | `localized_about_de` at `/localized-spaces/de/ueber` |

The source definitions are in `hyperbricks/75-localized-spaces.hyperbricks.yaml`.
They import `spaces/<source>/index.hyperbricks.yaml`; each managed index imports
its sibling instance files. The shared markup is in
`templates/patterns/localized-spaces-page.html`. No route is declared on either
source, so only the four instances are public pages.

## Editing ownership

`editable.heading` on a source's `content` template grants editing of
`content.values.heading` on its Spaces. The same applies to `intro`,
`section_heading`, and `section_copy`. The navigation URLs and labels are ordinary
template values, deliberately outside this example's editable allowlist. The
German instances override them to keep each language's links together.

The instance sets `htmltag: '<html lang="de">'` and a localized `head.meta`
description. The English instance uses `lang="en"`. Page titles and descriptions
are instance metadata; translated body text stays under `content.values`.
Template values that contain child components can execute while the parent
renders, even when the template no longer prints that value. Remove an unused
component-valued navigation field when replacing it with ordinary URL and label
values; a stale `menu` can still report missing sections.

## Creating this shape

Create a module with `hyperbricks init -m demo`, then use `author` for a new
named source root and `space` for each inheriting instance. First discover the
initialized project's revision, then prepare an `add-root` spec using that
revision. Both creation commands support a preview so you can inspect generated
YAML and managed imports before writing:

```sh
hyperbricks author context -m demo --list --json
hyperbricks author apply -m demo --spec localized-home-source.json --dry-run --json
```

The spec names `add-root`, `hypermedia`, `localized_home_source`, and
`localized.hyperbricks.yaml`; use the `revision` returned by `context`. Apply the
same spec without `--dry-run` to create the source. Add its shared template,
nested `editable` fields, and defaults before generating instances. Then discover
sources and preview each Space:

```sh
hyperbricks space -m demo --list --json
hyperbricks space -m demo --source localized_home_source \
  --name localized_home_de --title 'Deutsche Startseite' \
  --route de --dry-run --json
```

Run the same `space` command without `--dry-run` to create the instance, and
repeat for the other language and page source. The command copies editable
defaults into each instance and creates the import files. Adapt the generated
values, `htmltag`, and metadata for each language. Quote YAML prose containing
`: `, as in `intro: "Pattern: a reusable page source."`.

Before starting the server, set the module's shared developer credentials in the
same terminal:

```sh
export HB_DEVELOPER_USER=developer
export HB_DEVELOPER_PASSWORD='choose-a-long-password'
```

Replace the password placeholder with your own password. This module and new
modules created by `hyperbricks init` read these variables through
`hyperbricks.development.dashboard.credentials`. There is no default developer
account.

Developer credentials can also be set directly in `package.hyperbricks.yaml`.
Replace only `credentials` under `hyperbricks.development.dashboard`, leaving
`enabled` and the other settings unchanged:

```yaml
credentials:
  user: developer
  password: choose-a-long-password
```

With direct values, the environment exports above are not needed. Choose your
own password, restart the server, and use these values to log in. The password
is stored as plain text; do not commit real credentials to a shared repository.

Empty credentials produce **Developer interface unavailable: credentials
are not configured**; configure both values and restart. Open the configured Spaces
URL directly to log in, then reload the public page to discover its editor links.
The same login protects Overview, Errors and contextual editing, independently of
the `spaces.write` setting. See `docs/SPACES.md`, Development Configuration, in
the repository root for the complete access requirements.

This pattern module enables `hyperbricks.development.frontend_editing.spaces.write`
for local development. Every localized page, the pattern index, and the guide
sidebar show a **Manage Spaces** link when the editor API confirms writes are
available. The link stays hidden in
live or static output and when editor writes are disabled. The editor route comes
from the package configuration. For another module, enable writes explicitly and
restart the server. Saving persists YAML; the existing watcher or a restart
updates runtime configuration, and the browser still needs refreshing.

## Verify

Request all four routes directly and check their `html lang`, title, navigation
targets, translated copy, and `X-Hyperbricks-Render-Error-Count: 0`. Check the
Spaces catalog for both sources and all four instances. A `200 OK` alone does not
prove that nested components rendered successfully; use
`/__hyperbricks/render-diagnostics` when the error-count header is nonzero.

The patterns module includes unrelated plugin and action routes. Its default
static export discovers those routes too; a failed full-module export does not
isolate this pattern. Use a running development server to check these four pages,
or export them from a separate static-ready module.
