# Marker resolver test

A regression fixture for YAML `var`, `file`, and `path` resolvers. Its three
routes show how source values and configured directory paths reach rendered output.

- `/test_001` reads `resources/test.txt` through a `file` resolver and returns
  `hello world!` as a fragment.
- `/test_002` and `/test_003` inherit the same template, printing the sample
  `appName` and `version` variables together with values from `path.base`.

When started from the repository root, the expected bases are `module_root` =
`modules`, `root` = `./`, and `module` = `modules/markers-test`. The `resources`,
`templates`, `static`, and `hyperbricks` bases resolve to their configured module
subdirectories. The output label `template` uses the plural `templates` base.

The [marker test runner](../../scripts/run_marker_tests.sh) starts this module
on port `8085`, requests all three routes, and compares their output after
normalizing whitespace. It uses `go run ./cmd/hyperbricks` by default and stops
the test process afterwards. No plugins or external services are needed.
