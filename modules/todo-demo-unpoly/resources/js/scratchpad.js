// Notes are saved on every edit so even an immediate reload preserves them.
export function setupScratchpad(moduleName) {
  const field = document.getElementById("scratchpad");
  const status = document.getElementById("scratchpad-status");
  if (!field || !status) return;
  const key = `${moduleName}:scratchpad:v1`;
  const report = (message) => {
    if (status.textContent !== message) status.textContent = message;
  };
  try {
    field.value = localStorage.getItem(key) || "";
    report("Automatically saved in this browser.");
  } catch {
    report("Browser storage is unavailable. This note will not survive a reload.");
  }
  field.addEventListener("input", () => {
    try {
      localStorage.setItem(key, field.value);
      report("Automatically saved in this browser.");
    } catch {
      report("This note could not be saved. Copy it before reloading.");
    }
  });
  window.addEventListener("storage", (event) => {
    if (event.storageArea !== localStorage || (event.key !== key && event.key !== null)) return;
    field.value = event.newValue || "";
    report("Automatically saved in this browser.");
  });
}
