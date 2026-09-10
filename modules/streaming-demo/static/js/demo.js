// HTMX performs every request and panel swap. This code only controls the demo
// buttons and records when real SSE messages arrive; it never simulates progress.
const start = document.getElementById("start-stream");
const stop = document.getElementById("stop-stream");
const status = document.getElementById("connection-status");
const received = document.getElementById("received-updates");
let startedAt = 0;
let count = 0;
let stopped = false;
let failed = false;

function setBusy(busy) {
  start.disabled = busy;
  stop.disabled = !busy;
}

start.addEventListener("htmx:before:request", () => {
  startedAt = performance.now();
  count = 0;
  stopped = false;
  failed = false;
  received.replaceChildren();
  setBusy(true);
  status.textContent = "Connecting…";
});

start.addEventListener("htmx:sse:after:message", () => {
  count += 1;
  const title = document.querySelector("#stream-panel h2, #stream-panel h3")?.textContent || "Update received";
  const item = document.createElement("li");
  item.textContent = `${((performance.now() - startedAt) / 1000).toFixed(1)}s — ${title}`;
  received.append(item);
  status.textContent = `Receiving — ${count} of 4 updates.`;
});

start.addEventListener("htmx:sse:close", () => {
  setBusy(false);
  if (failed) return;
  status.textContent = stopped
    ? "Stopped. The last received update stays visible."
    : count === 4
      ? "Complete — four updates received in one response."
      : "The connection ended before all four updates arrived.";
});

function showError() {
  failed = true;
  setBusy(false);
  status.textContent = "The demo could not finish. Check that HyperBricks is running, then try again.";
}
start.addEventListener("htmx:sse:error", showError);
start.addEventListener("htmx:error", showError);
start.addEventListener("htmx:response:error", showError);

stop.addEventListener("click", () => {
  stopped = true;
  window.htmx.trigger(start, "htmx:abort");
});
