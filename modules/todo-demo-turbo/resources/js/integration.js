import * as Turbo from "../vendor/turbo-8.0.23.js";

// Ordinary page links remain ordinary navigation. Only the list uses a frame.
Turbo.session.drive = false;

export function renderPanel(url) {
  const frame = document.getElementById("todo-panel");
  return new Promise((resolve, reject) => {
    const controller = new AbortController();
    const handlers = {
      "turbo:before-fetch-request": (event) => {
        event.detail.fetchOptions.signal = controller.signal;
      },
      "turbo:before-fetch-response": (event) => {
        if (!event.detail.fetchResponse.response.ok) {
          event.preventDefault();
          finish(new Error("The server could not render the task list."));
        }
      },
      "turbo:frame-load": () => finish(),
      "turbo:frame-missing": (event) => {
        event.preventDefault();
        finish(new Error("The response did not contain the task frame."));
      },
      "turbo:fetch-request-error": (event) => {
        event.preventDefault();
        finish(new Error("The task request failed."));
      }
    };
    const timer = setTimeout(() => {
      controller.abort();
      finish(new Error("The task request timed out."));
    }, 10000);
    function finish(error) {
      clearTimeout(timer);
      for (const [name, handler] of Object.entries(handlers)) frame.removeEventListener(name, handler);
      if (error) reject(error); else resolve();
    }
    for (const [name, handler] of Object.entries(handlers)) frame.addEventListener(name, handler);
    frame.src = url;
  });
}
