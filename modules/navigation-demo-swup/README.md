# After Hours — Swup navigation demo

A text-only guide to four fictional evening venues. HyperBricks renders six complete pages; Swup 4.10.0 animates navigation between them.

## Run

From the repository root:

```sh
go run ./cmd/hyperbricks start -m navigation-demo-swup
```

Open http://localhost:8125/. The library is vendored locally and native esbuild bundles JavaScript and CSS. No npm install or separate server is required.

## Pages

- `/` — Explore
- `/night-owl-cafe`
- `/side-b-records`
- `/skyline-cinema`
- `/last-bite`
- `/how-it-works` — developer walkthrough of rendering, menus, transitions, and source files

## How it is built

Each page declares `section: guide_navigation` and an `index`. The `menu` component in `hyperbricks/partials/navigation.hyperbricks.yaml` generates the navigation using `sort: index`, including `aria-current="page"` for the active route. There is no JavaScript menu registry.

The developer page uses a separate `developer_navigation` section at the top-right of the header.

Swup replaces `#swup`, `#guide-navigation`, and `#developer-navigation` from the complete server response. The header shell and footer stay in place. Shared templates and esbuild assets follow the existing module pattern. Venue content is supplied through template values in `hyperbricks/app.hyperbricks.yaml`.

CSS supplies a short fade and an 8px entrance with a small stagger on the directory rows. The OS reduced-motion preference disables animated visits, including when the preference changes while the page is open. A page-view hook focuses the new main region without scrolling and announces the page title. Swup handles document titles and browser history; default nonanimated history visits retain native scroll restoration. Cache is disabled so development edits appear on the next visit.

Every link works without JavaScript. Direct URLs and reloads return full HTML. External footer links use ordinary browser navigation. There are no forms, storage, API actions, image assets, or frontend-rendered pages.

## Verify

Open each route directly; follow the section menu and Next stop links; check the title and active menu after each visit. Check Back/Forward, reload, narrow screens, keyboard navigation, reduced motion, and operation without JavaScript. See VENDOR.md for the library source and license.
