import {load} from "js-yaml";

export const MAX_LIVE_YAML_LENGTH = 1024 * 1024;

export function detectLineSeparator(source) {
  const match = String(source ?? "").match(/\r\n|\r|\n/);
  return match ? match[0] : "\n";
}

export function yamlDiagnostic(source) {
  const text = String(source ?? "");
  if (text.length > MAX_LIVE_YAML_LENGTH) {
    return {
      skipped: true,
      message: "Live syntax checking is skipped for configurations larger than 1 MiB."
    };
  }
  try {
    load(text, {
      filename: "package.hyperbricks.yaml",
      maxDepth: 100,
      maxAliases: 1000
    });
    return null;
  } catch (error) {
    const mark = error && typeof error === "object" ? error.mark : null;
    const position = Number.isInteger(mark?.position)
      ? Math.min(Math.max(mark.position, 0), text.length)
      : 0;
    const line = Number.isInteger(mark?.line) ? mark.line + 1 : 1;
    const column = Number.isInteger(mark?.column) ? mark.column + 1 : 1;
    const message = String(error?.reason || error?.message || "Invalid YAML").split("\n", 1)[0];

    return {
      message,
      line,
      column,
      from: position,
      to: Math.min(position + 1, text.length)
    };
  }
}
