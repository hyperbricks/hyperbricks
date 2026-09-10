import Swup from "../vendor/swup-4.10.0.js";

// Keep page navigation ordinary; Swup visits update only the task container.
const swup = new Swup({ containers: ["#todo-panel"], cache: false, linkSelector: "a[data-todo-swup]", animationSelector: false });
let pending;
swup.hooks.replace("page:load", (visit) => ({ url: visit.to.url, html: visit.meta.panelHtml }));
// A panel update should not scroll the surrounding page.
swup.hooks.replace("content:scroll", () => {});
swup.hooks.on("visit:end", () => { pending?.resolve(); pending = undefined; });
// Keep failures in the app instead of Swup's default full-page fallback.
swup.hooks.replace("visit:fail", (visit, { error }) => { pending?.reject(error); pending = undefined; });

export async function renderPanel(url) {
  if (pending) return Promise.reject(new Error("A task update is already running."));
  // Fetch before starting a visit: failed requests leave the current panel intact.
  const response = await fetch(url, {
    headers: { "X-Requested-With": "swup", Accept: "text/html" },
    cache: "no-store", signal: AbortSignal.timeout(10000)
  });
  if (!response.ok) throw new Error("The task request failed.");
  const html = await response.text();
  if (!new DOMParser().parseFromString(html, "text/html").querySelector("#todo-panel")) {
    throw new Error("The response did not contain the task panel.");
  }
  return new Promise((resolve, reject) => {
    pending = { resolve, reject };
    // Load fragment HTML through page:load without putting task data in history.
    swup.navigate(location.pathname, { animate: false, history: "replace", meta: { panelHtml: html } });
  });
}
