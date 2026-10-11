import "../vendor/htmx-4.0.0.js";

// HTMX owns requests and swaps. This small adapter handles document state
// that cannot be expressed in a fragment response.
const detail = document.getElementById("detail-data");
const retry = document.getElementById("retry-load");
const feedback = document.getElementById("cart-feedback");
const panelFocus = new WeakMap();
document.addEventListener("htmx:before:swap", event => {
  const { ctx } = event.detail;
  const panel = ctx.target;
  if (!["catalog-panel", "cart-panel", "checkout-content"].includes(panel?.id)) return;
  const active = document.activeElement;
  if (panel.contains(active)) panelFocus.set(ctx, { previous: active, panelID: panel.id });
});
document.addEventListener("htmx:after:swap", event => {
  const { ctx } = event.detail;
  if (!panelFocus.has(ctx)) return;
  const { previous, panelID } = panelFocus.get(ctx);
  panelFocus.delete(ctx);
  // HTMX restores surviving controls. Only repair focus lost with the old panel;
  // leave the search field or any control the user moved to undisturbed.
  const panel = document.getElementById(panelID);
  const active = document.activeElement;
  // A just-disabled replacement can still report as active during after:swap,
  // before the browser moves focus to the body.
  const lostFocus = active === document.body || active === previous || !active?.isConnected ||
    (panel?.contains(active) && (active.disabled || active.hidden));
  if (!lostFocus) return;
  const candidates = [previous.id];
  if (panelID === "catalog-panel") candidates.push("retry-load", "page-status", "result-count");
  if (panelID === "cart-panel") candidates.push("cart-summary", "cart-empty");
  if (panelID === "checkout-content") candidates.push("checkout-confirmation", "checkout-empty", "checkout-details");
  for (const id of candidates) {
    const target = document.getElementById(id);
    if (target && panel?.contains(target) && !target.disabled && !target.hidden) {
      target.focus({ preventScroll: true });
      if (document.activeElement === target) return;
    }
  }
});
let feedbackTimer;
document.addEventListener("htmx:after:swap", event => {
  if (event.detail.ctx.target !== feedback) return;
  clearTimeout(feedbackTimer);
  feedbackTimer = setTimeout(() => feedback.replaceChildren(), 5000);
});
if (detail && retry) {
  retry.setAttribute("hx-get", "/catalog-detail-feed" + location.search);
  const error = document.getElementById("load-error");
  const updateDetail = () => {
    const article = detail.querySelector(".detail-layout");
    const missing = detail.querySelector("[data-item-not-found]");
    error.hidden = Boolean(article || missing);
    retry.hidden = Boolean(missing);
    if (missing) document.title = "Item not found — FORM";
    if (article) document.title = article.querySelector("h1").textContent + " — FORM";
  };
  document.addEventListener("htmx:after:swap", event => {
    if (event.detail.ctx.target === detail) updateDetail();
  });
  window.addEventListener("pageshow", event => {
    if (event.persisted) retry.click();
  });
  updateDetail();
} else {
  document.addEventListener("click", event => {
    if (!event.target.closest("#clear-filters, #reset-search")) return;
    const search = document.getElementById("search");
    const category = document.getElementById("category");
    search.value = "";
    category.value = "";
    search.focus();
    search.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
