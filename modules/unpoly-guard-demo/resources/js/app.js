const up = window.up;
const message = (text) => { document.getElementById("message").textContent = text; };
const action = (url, params = {}) => up.request(url, {method:"POST", params, headers:{"X-Demo-Action":"true"}, cache:false, timeout:10000});
document.getElementById("login-form")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  try { await action("/auth/login", new FormData(event.target)); location.assign("/"); }
  catch (error) { message(error.status === 401 ? "Incorrect username or password." : "Sign in failed. Try again."); }
});
document.getElementById("expire-session")?.addEventListener("click", async () => {
  try { await action("/auth/logout"); message("Session ended. The next protected request will open Sign in."); }
  catch { message("Could not end the session."); }
});
document.getElementById("load-panel")?.addEventListener("click", async (event) => {
  event.target.disabled = true;
  try {
    const response = await up.request("/fragments/private", {headers:{"X-Demo-Client":"unpoly"}, target:"#protected-panel", cache:false, timeout:10000});
    await up.render({response, target:"#protected-panel", history:false, fallback:false});
    message("Protected fragment received. The rest of the page stayed in place.");
  } catch (error) {
    if (error.status === 401) location.assign("/login");
    else if (error.status === 403) location.assign("/forbidden");
    else message("The request failed. Try again.");
  } finally { event.target.disabled = false; }
});

const note = document.getElementById("note");
if (note) {
  try { note.value = localStorage.getItem("unpoly-guard-demo:note") || ""; }
  catch { message("Note storage is unavailable."); }
  note.addEventListener("input", () => {
    try { localStorage.setItem("unpoly-guard-demo:note", note.value); }
    catch { message("Your note could not be saved. Copy it before leaving."); }
  });
}
