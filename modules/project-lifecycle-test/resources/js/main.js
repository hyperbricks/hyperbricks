import htmx from "../vendor/htmx-4.0.0.js";

window.htmx = htmx;
// Back/Forward reload canonical pages, including fresh request-bound results.
htmx.config.history = "reload";

function updatePage() {
  const section = document.querySelector("#main-content [data-page-title]");
  if (section) document.title = `${section.dataset.pageTitle} · Project Lifecycle Fixture`;
  document.querySelectorAll(".navigation a").forEach((link) => {
    const active = link.pathname === "/"
      ? location.pathname === "/"
      : location.pathname === link.pathname || location.pathname.startsWith(`${link.pathname}/`);
    if (active) link.setAttribute("aria-current", "page");
    else link.removeAttribute("aria-current");
  });
}

document.addEventListener("DOMContentLoaded", updatePage);
document.addEventListener("htmx:after:settle", (event) => {
  updatePage();
  if (event.detail?.task?.target?.id === "main-content") {
    document.getElementById("main-content").focus({ preventScroll: true });
    window.scrollTo(0, 0);
  }
});
