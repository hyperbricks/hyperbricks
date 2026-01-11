# Deploy Guide

This guide covers building and running deployable Hyperbricks archives (.hra/.zip) using the deploy workflow.

## Build

Create a deploy archive for a module:

```bash
hyperbricks build --hra -m <module>
```

Optional flags:
- `--zip` to create a `.zip` instead of `.hra`
- `--out <dir>` to change the deploy folder (default: `deploy`)
- `--force` to rebuild even when the source hash is unchanged
- `--replace[=<build_id>]` to replace the current build or a specific build ID

Build outputs:
- Archive: `deploy/<module>/<module>-<moduleversion>-<build_id>.hra|.zip`
- Index: `deploy/<module>/hyperbricks.versions.json`

The index includes:
- `current` build ID pointer
- `versions[]` entries with file path, build metadata, and `source_hash`

## Run From Deploy

Start the server using the current deploy build:

```bash
hyperbricks start --deploy -m <module>
```

Optional flags:
- `--deploy-dir <dir>` to point at a custom deploy folder

Runtime extraction:
- Archives are extracted to `deploy/<module>/runtime/<build_id>/`
- The server reads `package.hyperbricks` from that runtime directory

If you need a fresh extraction, remove the runtime folder and start again:

```bash
rm -rf deploy/<module>/runtime/<build_id>
hyperbricks start --deploy -m <module>
```

## Rollbacks

To roll back, set `current` in `deploy/<module>/hyperbricks.versions.json` to an older build ID and restart with `--deploy`.

## Required Metadata

Each module must have `hyperbricks.metadata.moduleversion` in `package.hyperbricks`.
The `build` command injects or updates dynamic fields on archive creation.
