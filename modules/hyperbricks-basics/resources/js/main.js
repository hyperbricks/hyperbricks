import htmx from "../vendor/htmx-2.0.10.js";

window.htmx = htmx;
// History should refetch canonical pages, including fresh request-bound results.
htmx.config.historyCacheSize = 0;
htmx.config.historyRestoreAsHxRequest = false;

function updatePage() {
  const section = document.querySelector("#main-content [data-page-title]");
  if (section) document.title = `${section.dataset.pageTitle} · Project Desk`;
  document.querySelectorAll(".navigation a").forEach((link) => {
    const active = link.pathname === "/"
      ? location.pathname === "/"
      : location.pathname === link.pathname || location.pathname.startsWith(`${link.pathname}/`);
    if (active) link.setAttribute("aria-current", "page");
    else link.removeAttribute("aria-current");
  });
}

document.addEventListener("DOMContentLoaded", updatePage);
document.addEventListener("htmx:afterSettle", (event) => {
  updatePage();
  if (event.detail.target?.id === "main-content") {
    document.getElementById("main-content").focus({ preventScroll: true });
    window.scrollTo(0, 0);
  }
});
document.addEventListener("htmx:historyRestore", updatePage);
