import Swup from "../vendor/swup-4.10.0.js";

// Contextual editing uses full-page navigation to load and remove its editor UI.
if (new URLSearchParams(window.location.search).get("edit") !== "true") {
  // All containers come from complete HyperBricks pages. The menu owns active links.
  const swup = new Swup({
    containers: ["#swup", "#guide-navigation", "#developer-navigation"],
    animationSelector: ".transition-page, .transition-item",
    cache: false
  });
  const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  swup.hooks.on("visit:start", (visit) => {
    if (reducedMotion.matches) visit.animation.animate = false;
  });
  swup.hooks.on("page:view", () => {
    // Avoid moving the scroll position restored by browser Back/Forward.
    const main = document.querySelector("#swup");
    document.documentElement.lang = main.dataset.pageLanguage;
    main.focus({ preventScroll: true });
    document.querySelector("#page-announcement").textContent = document.title;
  });
}
