import test from "node:test";
import assert from "node:assert/strict";
import {Compartment, EditorState} from "@codemirror/state";

import {
  detectLineSeparator,
  MAX_LIVE_YAML_LENGTH,
  yamlDiagnostic
} from "./src/deploy_yaml_diagnostics.mjs";
import {normalizeClipboardLineSeparators} from "./src/deploy_yaml_editor.mjs";

test("YAML diagnostics accept free variables without rewriting the source", () => {
  const source = [
    "# keep this comment",
    "hyperbricks:",
    "  mode: \"live\"",
    "",
    "vars:",
    "  free_value: \"001\"",
    ""
  ].join("\r\n");

  assert.equal(yamlDiagnostic(source), null);
  assert.equal(detectLineSeparator(source), "\r\n");
});

test("YAML diagnostics expose stable one-based coordinates", () => {
  const source = "hyperbricks:\r\n  mode: live\r\n  vars: [one,\r\n";
  const diagnostic = yamlDiagnostic(source);

  assert.ok(diagnostic);
  assert.match(diagnostic.message, /indentation|flow collection|end of the stream/i);
  assert.equal(diagnostic.line, 4);
  assert.equal(diagnostic.column, 1);
  assert.equal(diagnostic.from, source.length);
  assert.equal(diagnostic.to, source.length);
});

test("a valid edit clears a previous YAML diagnostic", () => {
  assert.ok(yamlDiagnostic("items: [one,\n"));
  assert.equal(yamlDiagnostic("items:\n  - one\n"), null);
});

test("line-ending detection defaults to LF and recognizes legacy CR", () => {
  assert.equal(detectLineSeparator("key: value"), "\n");
  assert.equal(detectLineSeparator("a:\n  b: c\n"), "\n");
  assert.equal(detectLineSeparator("a:\r  b: c\r"), "\r");
});

test("CodeMirror extraction preserves a configured CRLF document exactly", () => {
  const lineSeparator = new Compartment();
  const initialSource = "hyperbricks:\n  mode: development\n";
  let state = EditorState.create({
    doc: initialSource,
    extensions: [
      lineSeparator.of(EditorState.lineSeparator.of(detectLineSeparator(initialSource)))
    ]
  });
  const crlfSource = "# keep this comment\r\nhyperbricks:\r\n  mode: \"live\"\r\n";

  state = state.update({
    effects: lineSeparator.reconfigure(
      EditorState.lineSeparator.of(detectLineSeparator(crlfSource))
    )
  }).state;
  state = state.update({
    changes: {from: 0, to: state.doc.length, insert: crlfSource}
  }).state;

  assert.equal(state.sliceDoc(), crlfSource);
  assert.notEqual(state.doc.toString(), crlfSource, "Text.toString() normalizes line endings");

  const lfSource = "hyperbricks:\n  mode: development\n";
  state = state.update({
    effects: lineSeparator.reconfigure(
      EditorState.lineSeparator.of(detectLineSeparator(lfSource))
    )
  }).state;
  state = state.update({
    changes: {from: 0, to: state.doc.length, insert: lfSource}
  }).state;

  assert.equal(state.sliceDoc(), lfSource);
});

test("multiline clipboard input becomes real lines with the configured separator", () => {
  const state = EditorState.create({
    doc: "hyperbricks:\r\n  mode: live\r\n",
    extensions: [EditorState.lineSeparator.of("\r\n")]
  });
  const pasted = normalizeClipboardLineSeparators("vars:\n  free: yes\n", state);
  const updated = state.update({changes: {from: 0, insert: pasted}}).state;

  assert.equal(pasted, "vars:\r\n  free: yes\r\n");
  assert.equal(updated.doc.lines, 5);
  assert.equal(updated.sliceDoc(), "vars:\r\n  free: yes\r\nhyperbricks:\r\n  mode: live\r\n");
});

test("large configurations skip live parsing without becoming invalid", () => {
  const diagnostic = yamlDiagnostic("x".repeat(MAX_LIVE_YAML_LENGTH + 1));
  assert.equal(diagnostic.skipped, true);
  assert.match(diagnostic.message, /larger than 1 MiB/);
});
