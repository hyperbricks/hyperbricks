import "../vendor/htmx-4.0.0.js";

let updates = 0;
document.addEventListener("htmx:after:swap", () => {
  updates += 1;
  document.getElementById("update-status").textContent =
    `Card updated ${updates} time${updates === 1 ? "" : "s"}.`;
});
