# Invalid YAML source fixture

An intentionally broken regression fixture for YAML loading and development
diagnostics. This is not an application starter or an example of valid source.

`hyperbricks/broken.hyperbricks.yaml` ends with an unfinished sequence at line 4.
It is kept beside valid page and fragment sources to check that one malformed
file does not prevent the valid sources from being processed.

The regression test checks that `/valid-route` is indexed, the broken route is
not indexed, and the load diagnostic identifies the relative file and line.
It also checks that requesting `/broken-route` returns a development error
response with the source diagnostic, without exposing the absolute repository path.

The package selects development mode with file watching and reload enabled;
the dashboard and frontend error overlay are disabled. No plugins are required.

Do not repair the malformed file as routine cleanup: its syntax error is the
test input. Source-validation failures for this module are expected.
