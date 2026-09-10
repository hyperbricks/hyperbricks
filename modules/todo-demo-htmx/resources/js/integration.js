import htmx from "../vendor/htmx-4.0.0.js";

htmx.config.defaultTimeout = 10000;
htmx.config.history = false;

export function renderPanel(url) {
  return htmx.ajax("GET", url, {
    target: "#todo-panel",
    swap: "outerHTML"
  });
}
