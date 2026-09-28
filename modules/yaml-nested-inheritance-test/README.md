# Nested YAML inheritance regression fixture

This runnable module demonstrates the nested inheritance behavior fixed by
commit `08a72434c9dbcbce4afeb90ea06043de86c3c019`. It starts with the smallest
reproduction, then adds focused edge cases for recursive map merging, ordinary
same-name child merging, inherited reference targets, deep dotted paths, and
unrelated sibling preservation.

The important source is
[`hyperbricks/nested-inheritance.hyperbricks.yaml`](hyperbricks/nested-inheritance.hyperbricks.yaml).
The automated HTTP check lives in
[`scripts/test_modules.py`](../../scripts/test_modules.py).

## Run it

From the repository root, start this checkout's runtime:

```sh
go run ./cmd/hyperbricks start -m yaml-nested-inheritance-test --non-interactive
```

Open the index, then inspect or request the six result pages:

```sh
curl http://localhost:8133/
curl http://localhost:8133/reference
curl http://localhost:8133/copied
curl http://localhost:8133/overridden
curl http://localhost:8133/direct-merge
curl http://localhost:8133/chained
curl http://localhost:8133/deep-dotted
```

The index page links to all six result pages for a quick browser walkthrough.

The expected visible values are:

| Route | Behavior | Distinguishing result |
| --- | --- | --- |
| `/reference` | Baseline template replacing the shell's `content` tree | `Version: 1.0.0`, `Channel: stable`, `Override: inherited default` |
| `/copied` | Exact `reference_page.content` dotted-path copy | Same values as `/reference` |
| `/overridden` | Dotted-path copy with recursive local map merge | Preserves version, changes channel to `preview`, changes override to `works` |
| `/direct-merge` | Same-name child without a local `inherit` | Uses the normal merge path; channel becomes `candidate` |
| `/chained` | Dotted reference to an already inherited and overridden child | Preserves `preview` and `works`, changes version to `2.0.0` |
| `/deep-dotted` | Multi-segment `component_library.panels.release_card` lookup | Preserves owner `core`, changes stage to `customized` |

Every inherited page also renders `Shell sibling preserved`. That marker proves
that replacing or merging `content` does not remove an unrelated child inherited
from `page_shell`.

Run only this module's automated smoke check with:

```sh
python3 scripts/test_modules.py --module yaml-nested-inheritance-test
```

It starts the module on an available local port and checks every result above,
including the inherited shell sibling. The module is also included in
`./tests.sh --with-modules`.

## The inheritance chain

The source uses three layers:

1. `page_shell` defines a Hypermedia page with a `content` child of type `tree`
   and an unrelated `shell_marker` sibling.
2. `reference_page` inherits the shell and replaces `content` with a `template`.
   Its `values` map supplies the heading, a nested `release` map, and default
   override text.
3. `copied_page` and `overridden_page` each inherit `page_shell`, then declare a
   same-named local `content` child with `inherit: reference_page.content`.

`overridden_page` adds a local `values` map. HyperBricks resolves
`reference_page.content` to the template first, then recursively merges the
local map into the referenced values. The local `heading`, `release.channel`,
and `override` win; the unmentioned `release.version` remains `1.0.0`.

## Advanced cases

### Same-name child without local inheritance

`direct_merge_page` inherits `reference_page` and supplies local `content.values`
without another `inherit`. This exercises the branch intentionally left
unchanged by the fix: same-named children without a local inheritance reference
still deep-merge. The inherited template and version remain available while
local values change the heading, channel, and override text.

### Multi-hop inherited reference

`chained_page.content` inherits `overridden_page.content`. That target was
itself created by resolving `reference_page.content` and merging local values.
The chain must retain those earlier results (`preview` and `works`) while a new
nested override changes only the version to `2.0.0`.

### Deep dotted lookup

`deep_dotted_page.content` resolves
`component_library.panels.release_card`, traversing two named children below the
root. Its local nested map changes `metadata.stage` while preserving
`metadata.owner`. This distinguishes path traversal from recursive value
merging: both must succeed for the rendered markers to appear.

### Unrelated sibling preservation

All inherited pages retain `page_shell.shell_marker`. This checks that replacing
the same-named `content` node does not replace the shell's complete child list or
disturb unrelated inherited siblings.

## What the fix changed

Materializing `overridden_page` starts by copying `page_shell`, so the working
page already has a child named `content`. The local page also has a child named
`content`, but that local child carries its own dotted inheritance reference.

Before the fix, the general same-name child merge ran immediately. That merge
kept the shell child's `tree` type but did not carry the local child's
`inherit` reference forward. Because the reference was lost before resolution,
the intended template and its values never became the page content.

The fix makes this decision explicit: when a same-named local child has a
non-empty `inherit`, HyperBricks keeps that local node intact instead of merging
it into the inherited child. The normal resolver can then dereference
`reference_page.content`; afterward, the local values merge onto the resolved
template. A same-named child without its own `inherit` still follows the normal
deep-merge path.

This fixture covers the runtime result. The parser-level regression tests in
[`pkg/yaml-parser/parser_test.go`](../../pkg/yaml-parser/parser_test.go) assert
the corresponding materialized maps directly.
