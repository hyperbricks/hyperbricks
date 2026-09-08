import { renderPanel } from "./integration.js";
import { setupScratchpad } from "./scratchpad.js";

const app = document.getElementById("todo-app");
if (app) {
  setupScratchpad(document.body.dataset.module);
  const key = `${document.body.dataset.module}:tasks:v1`;
  const form = document.getElementById("add-task");
  const title = document.getElementById("task-title");
  const status = document.getElementById("app-status");
  let tasks = [];
  let filter = "all";
  let busy = false;
  let storageWarning = "";

  function validate(value) {
    if (!Array.isArray(value) || value.length > 20) throw new Error("This demo supports up to 20 tasks.");
    const ids = new Set();
    return value.map((task) => {
      if (!task || typeof task.id !== "string" || !/^[a-zA-Z0-9-]{1,64}$/.test(task.id) || ids.has(task.id) || typeof task.title !== "string" || !task.title.trim() || task.title.length > 120 || typeof task.done !== "boolean") {
        throw new Error("The saved task list has an invalid format.");
      }
      ids.add(task.id);
      return { id: task.id, title: task.title.trim(), done: task.done };
    });
  }

  try {
    const saved = localStorage.getItem(key);
    if (saved !== null) tasks = validate(JSON.parse(saved));
  } catch {
    storageWarning = "Saved tasks could not be read. Existing storage has not been changed.";
  }

  function setBusy(value) {
    busy = value;
    app.setAttribute("aria-busy", String(value));
    app.querySelectorAll("button, input").forEach((element) => { element.disabled = value; });
  }

  async function update(next, nextFilter = filter, persist = true) {
    if (busy) return false;
    const previous = tasks;
    let candidate;
    try { candidate = validate(next); }
    catch (error) { status.textContent = error.message; return false; }
    setBusy(true);
    status.textContent = "Updating your list…";
    const request = `${Date.now()}-${Math.random().toString(36).slice(2)}`;
    const query = new URLSearchParams({ state: JSON.stringify(candidate), filter: nextFilter, request });
    try {
      await renderPanel(`/fragments/tasks?${query}`);
      const result = document.querySelector("#todo-panel [data-render-result]");
      if (!result || result.dataset.request !== request) throw new Error("The server did not return the requested list.");
      const error = result.querySelector("[data-render-error]");
      if (error) throw new Error(error.textContent);
      tasks = candidate;
      filter = nextFilter;
      if (persist) {
        try { localStorage.setItem(key, JSON.stringify(tasks)); storageWarning = ""; }
        catch { storageWarning = "Browser storage is unavailable. Changes are kept only until you reload."; }
      }
      app.querySelectorAll("[data-filter]").forEach((button) => {
        if (button.tagName === "BUTTON") button.setAttribute("aria-pressed", String(button.dataset.filter === filter));
      });
      status.textContent = storageWarning || (persist ? "Saved in this browser." : "Your list is ready.");
      return true;
    } catch {
      status.textContent = "The list could not be updated. Your saved tasks are unchanged. Try Refresh list.";
      // Restore a checkbox the browser toggled before a failed request.
      app.querySelectorAll("[data-toggle]").forEach((checkbox) => {
        checkbox.checked = previous.find((task) => task.id === checkbox.dataset.toggle)?.done || false;
      });
      return false;
    } finally { setBusy(false); }
  }

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const value = title.value.trim();
    if (!value || busy) return;
    const id = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`;
    if (await update([...tasks, { id, title: value, done: false }])) {
      title.value = "";
      title.focus();
    }
  });
  app.addEventListener("change", (event) => {
    const id = event.target.dataset.toggle;
    if (id && !busy) update(tasks.map((task) => task.id === id ? { ...task, done: event.target.checked } : task));
  });
  app.addEventListener("click", (event) => {
    const button = event.target.closest("button");
    if (!button || busy) return;
    if (button.dataset.delete) update(tasks.filter((task) => task.id !== button.dataset.delete));
    else if (button.dataset.filter) update(tasks, button.dataset.filter, false);
    else if (button.id === "clear-completed") update(tasks.filter((task) => !task.done));
    else if (button.id === "retry-render") update(tasks, filter, false);
  });
  // Other tabs on this origin share localStorage; request fresh server HTML.
  window.addEventListener("storage", (event) => {
    if (event.key !== key || busy) return;
    try { update(event.newValue === null ? [] : validate(JSON.parse(event.newValue)), filter, false); }
    catch { status.textContent = "Another tab saved an invalid list. Your current list is unchanged."; }
  });
  update(tasks, filter, false);
}
