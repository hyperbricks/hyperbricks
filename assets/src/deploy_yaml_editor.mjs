/*!
Third-party software notices for the bundled HyperBricks deployment YAML editor:

CodeMirror state, view, commands, and language packages
Copyright (C) 2018-2021 by Marijn Haverbeke <marijn@haverbeke.berlin> and others
CodeMirror YAML language package
Copyright (C) 2024 by Marijn Haverbeke <marijn@haverbeke.berlin> and others
Lezer common, highlight, and LR packages
Copyright (C) 2018 by Marijn Haverbeke <marijn@haverbeke.berlin> and others
Lezer YAML package
Copyright (C) 2024 by Marijn Haverbeke <marijnh@gmail.com> and others
find-cluster-break
Copyright (C) 2024 by Marijn Haverbeke <marijn@haverbeke.berlin>
crelt
Copyright (C) 2020 by Marijn Haverbeke <marijn@haverbeke.berlin>
style-mod
Copyright (C) 2018 by Marijn Haverbeke <marijn@haverbeke.berlin> and others
w3c-keyname
Copyright (C) 2016 by Marijn Haverbeke <marijn@haverbeke.berlin> and others
js-yaml
Copyright (C) 2011-2015 by Vitaly Puzrin

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

import {Compartment, EditorState} from "@codemirror/state";
import {
  EditorView,
  highlightActiveLine,
  highlightActiveLineGutter,
  highlightSpecialChars,
  keymap,
  lineNumbers
} from "@codemirror/view";
import {defaultKeymap, history, historyKeymap, indentWithTab} from "@codemirror/commands";
import {
  bracketMatching,
  HighlightStyle,
  indentOnInput,
  indentUnit,
  syntaxHighlighting
} from "@codemirror/language";
import {yaml} from "@codemirror/lang-yaml";
import {tags} from "@lezer/highlight";
import {detectLineSeparator, yamlDiagnostic} from "./deploy_yaml_diagnostics.mjs";

const hyperBricksHighlightStyle = HighlightStyle.define([
  {tag: tags.comment, color: "var(--hb-yaml-comment)", fontStyle: "italic"},
  {tag: [tags.propertyName, tags.name], color: "var(--hb-yaml-key)"},
  {tag: [tags.string, tags.special(tags.string)], color: "var(--hb-yaml-string)"},
  {tag: [tags.number, tags.bool, tags.null], color: "var(--hb-yaml-literal)"},
  {tag: [tags.punctuation, tags.separator, tags.brace], color: "var(--hb-yaml-punctuation)"}
]);

// Let the enclosing native dialog own Escape, including when the editor has a
// selection. CodeMirror's default Escape binding would otherwise consume the
// first press to simplify that selection instead of closing the dialog.
const editorDefaultKeymap = defaultKeymap.filter((binding) => binding.key !== "Escape");

function readOnlyExtensions(readOnly) {
  return [
    EditorState.readOnly.of(readOnly),
    EditorView.editable.of(!readOnly)
  ];
}

export function normalizeClipboardLineSeparators(text, state) {
  return String(text ?? "").replace(/\r\n|\r|\n/g, state.lineBreak);
}

export function createDeployYAMLEditor({
  parent,
  doc = "",
  readOnly = false,
  onChange = () => {},
  onSave = () => {},
  onValidation = () => {}
}) {
  if (!parent) {
    throw new Error("A YAML editor parent element is required.");
  }

  const readOnlyCompartment = new Compartment();
  const lineSeparatorCompartment = new Compartment();
  const initialSource = String(doc ?? "");
  let validationTimer = null;
  function publishValidation(source) {
    const diagnostic = yamlDiagnostic(source);
    if (diagnostic?.skipped) {
      onValidation({valid: null, ...diagnostic});
    } else {
      onValidation(diagnostic
        ? {valid: false, ...diagnostic}
        : {valid: true});
    }
  }
  function scheduleValidation(source) {
    if (validationTimer !== null) {
      clearTimeout(validationTimer);
    }
    validationTimer = setTimeout(() => {
      validationTimer = null;
      publishValidation(source);
    }, 300);
  }

  const state = EditorState.create({
    doc: initialSource,
    extensions: [
      lineSeparatorCompartment.of(EditorState.lineSeparator.of(detectLineSeparator(initialSource))),
      lineNumbers(),
      highlightActiveLineGutter(),
      highlightActiveLine(),
      highlightSpecialChars(),
      history(),
      indentOnInput(),
      indentUnit.of("  "),
      bracketMatching(),
      syntaxHighlighting(hyperBricksHighlightStyle),
      yaml(),
      EditorView.contentAttributes.of({
        "aria-label": "Package configuration YAML",
        "aria-multiline": "true"
      }),
      EditorView.clipboardInputFilter.of(normalizeClipboardLineSeparators),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          const source = update.state.sliceDoc();
          onChange(source);
          scheduleValidation(source);
        }
      }),
      keymap.of([
        {
          key: "Mod-s",
          preventDefault: true,
          run: () => {
            onSave();
            return true;
          }
        },
        indentWithTab,
        ...editorDefaultKeymap,
        ...historyKeymap
      ]),
      readOnlyCompartment.of(readOnlyExtensions(Boolean(readOnly)))
    ]
  });

  const view = new EditorView({state, parent});
  publishValidation(initialSource);

  return {
    getValue() {
      return view.state.sliceDoc();
    },
    setValue(value) {
      const nextSource = String(value ?? "");
      if (nextSource === view.state.sliceDoc()) {
        return;
      }
      // Reconfigure first. If the line-separator effect and a CRLF insertion
      // share one transaction, CodeMirror parses the insertion with the old LF
      // separator and retains each carriage return as document content.
      view.dispatch({
        effects: lineSeparatorCompartment.reconfigure(
          EditorState.lineSeparator.of(detectLineSeparator(nextSource))
        )
      });
      view.dispatch({
        changes: {from: 0, to: view.state.doc.length, insert: nextSource}
      });
    },
    setReadOnly(nextReadOnly) {
      view.dispatch({
        effects: readOnlyCompartment.reconfigure(readOnlyExtensions(Boolean(nextReadOnly)))
      });
    },
    focus() {
      view.focus();
    },
    destroy() {
      if (validationTimer !== null) {
        clearTimeout(validationTimer);
        validationTimer = null;
      }
      view.destroy();
    }
  };
}

export {yamlDiagnostic};
