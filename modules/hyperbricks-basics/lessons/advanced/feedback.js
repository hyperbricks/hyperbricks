// The API status is rendered into this marker by save-result.html.
// Emit a refresh only after a successful save, not after validation or conflict.
document.addEventListener("htmx:after:swap", (event) => {
  const target = event.detail?.ctx?.target;
  if (!(target instanceof Element) || target.id !== "advanced-save-feedback") return;
  const result = target.querySelector("[data-project-save-status]");
  if (result?.dataset.projectSaveStatus === "200") {
    window.htmx.trigger(document.body, "basics-project-updated");
  }
});
