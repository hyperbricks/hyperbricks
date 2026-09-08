export function renderPanel(url) {
  return window.up.render({
    url,
    target: "#todo-panel",
    history: false,
    cache: false,
    fallback: false,
    timeout: 10000
  });
}
