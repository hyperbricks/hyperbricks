import "../vendor/htmx-4.0.0.js";

// HTMX owns requests and swaps. This small adapter handles document state
// that cannot be expressed in a fragment response.
const detail = document.getElementById("detail-data");
const retry = document.getElementById("retry-load");
const feedback = document.getElementById("cart-feedback");
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
