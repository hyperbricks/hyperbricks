import {readFileSync} from 'node:fs';
import {createHash, createHmac, randomFillSync} from 'node:crypto';
import vm from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';

const html = readFileSync(new URL('./deploy_dashboard.html', import.meta.url), 'utf8');
const source = html.match(/<script>([\s\S]*?)<\/script>/)[1];
const flush = () => new Promise(resolve => setImmediate(resolve));

function setup({saved = '', savedBase = '', savedView = '', blocked = false, mode = 'remote', apiError = '', networkError = '', responseGate} = {}) {
  const elements = new Map(), events = new Map(), requests = [];
  const storage = new Map(saved ? [['hbDeploySecret', saved]] : []);
  if (savedBase) storage.set('hbDeployBaseUrl', savedBase);
  if (savedView) storage.set('hbDeployView', savedView);
  const reply = {status: apiError ? 401 : 200, error: apiError, networkError};
  let focused = '', reloads = 0, moduleLoads = 0, pluginLoads = 0;
  for (const [, id] of html.matchAll(/\bid="([^"]+)"/g)) {
    const classes = new Set(id === 'menuPanel' ? ['hidden'] : []);
    const listeners = new Map(), attrs = new Map();
    elements.set(id, {
      value: '', textContent: '', disabled: false, hidden: false, open: false, files: [], listeners, attrs,
      addEventListener: (name, fn) => listeners.set(name, fn),
      setAttribute: (name, value) => attrs.set(name, value),
      removeAttribute: name => attrs.delete(name),
      focus: () => { focused = id; },
      reset: () => {},
      showModal() { this.open = true; },
      close() { this.open = false; },
      querySelector: () => ({textContent: ''}),
      classList: {
        add: name => classes.add(name),
        remove: name => classes.delete(name),
        contains: name => classes.has(name),
        toggle: (name, force = !classes.has(name)) => force ? classes.add(name) : classes.delete(name)
      }
    });
  }
  const context = vm.createContext({
    document: {
      body: {dataset: {mode}, classList: {add() {}}},
      getElementById: id => elements.get(id),
      addEventListener: (name, fn) => events.set(name, fn)
    },
    window: {
      location: {origin: 'http://localhost:9090', reload: () => { reloads++; }},
      crypto: {getRandomValues: bytes => randomFillSync(bytes)},
      addEventListener() {}
    },
    localStorage: {
      getItem: key => storage.get(key) ?? null,
      setItem: (key, value) => {
        if (blocked && key === 'hbDeploySecret') throw Error('storage blocked');
        storage.set(key, value);
      },
      removeItem: key => storage.delete(key)
    },
    URL, TextEncoder, Uint8Array, ArrayBuffer, crypto: {getRandomValues: bytes => randomFillSync(bytes)}, setInterval: () => 1,
    recordModuleLoad: async () => { moduleLoads++; },
    recordPluginLoad: async () => { pluginLoads++; },
    fetch: async (url, options) => {
      requests.push({url, ...options});
      if (responseGate) await responseGate;
      if (reply.networkError) throw Error(reply.networkError);
      return {ok: reply.status === 200, status: reply.status, json: async () => reply.error ? {error: reply.error} : {version: 'test'}};
    }
  });
  // Keep production event wiring and initialization, but avoid unrelated module rendering.
  vm.runInContext(source.replace(/\s+wireEvents\(\);\s+initDefaults\(\);\s*$/, ''), context);
  vm.runInContext('loadModules = recordModuleLoad; loadGlobalPlugins = recordPluginLoad; renderPluginModules = () => {}; renderPluginBuilds = () => {}; wireEvents(); initDefaults();', context);
  const element = id => elements.get(id);
  const fire = (id, name, event = {}) => element(id).listeners.get(name)({preventDefault() {}, stopPropagation() {}, ...event});
  const type = value => { element('secret').value = value; fire('secret', 'input'); };
  return {element, fire, type, events, storage, requests, reply, moduleLoads: () => moduleLoads, pluginLoads: () => pluginLoads, reloads: () => reloads, focused: () => focused, run: code => vm.runInContext(code, context)};
}

function assertSignedWith(request, secret) {
  const headers = request.headers;
  const hash = createHash('sha256').update('').digest('hex');
  const canonical = [request.method, new URL(request.url).pathname, hash, headers['X-HB-Timestamp'], headers['X-HB-Nonce']].join('\n');
  assert.equal(headers['X-HB-Signature'], createHmac('sha256', secret).update(canonical).digest('hex'));
}

test('typing is a draft; Set persists and uses the applied secret for requests', async () => {
  const ui = setup();
  assert.equal(ui.element('setSecret').disabled, true);
  ui.type('  test-secret  ');
  await flush();
  assert.equal(ui.storage.has('hbDeploySecret'), false);
  assert.equal(ui.requests.length, 0);
  assert.equal(ui.element('secretStatus').textContent, 'Changes not set.');
  assert.equal(ui.element('setSecret').disabled, false);
  await assert.rejects(ui.run('apiRequest("GET", apiPath("/status"))'), /Missing HMAC secret/);
  assert.equal(ui.requests.length, 0);
  ui.fire('setSecret', 'click');
  await flush();
  assert.equal(ui.storage.get('hbDeploySecret'), 'test-secret');
  assert.equal(ui.reloads(), 1);
  assert.equal(ui.element('secretStatus').textContent, 'Secret set.');
  assert.equal(ui.element('setSecret').disabled, true);
  assertSignedWith(ui.requests.at(-1), 'test-secret');
  ui.type('unapplied-change');
  await ui.run('refreshServerStatus()');
  assertSignedWith(ui.requests.at(-1), 'test-secret');
  assert.equal(ui.storage.get('hbDeploySecret'), 'test-secret');
  assert.equal(ui.reloads(), 1);
});

test('Enter applies a changed secret, but empty drafts cannot replace it', async () => {
  const ui = setup();
  ui.type('test-secret');
  ui.fire('secret', 'keydown', {key: 'Enter'});
  await flush();
  assert.equal(ui.storage.get('hbDeploySecret'), 'test-secret');
  assert.equal(ui.reloads(), 1);
  ui.type('   ');
  assert.equal(ui.element('setSecret').disabled, true);
  ui.fire('secret', 'keydown', {key: 'Enter'});
  assert.equal(ui.run('getSecret()'), 'test-secret');
  ui.type('test-secret');
  assert.equal(ui.element('secretStatus').textContent, 'Secret set.');
});

test('saved secrets are restored; Clear removes the saved, active and draft values', async () => {
  const ui = setup({saved: 'test-secret'});
  await flush();
  assert.equal(ui.element('secretStatus').textContent, 'Secret set.');
  assertSignedWith(ui.requests.at(-1), 'test-secret');
  ui.type('draft');
  ui.fire('clearSecret', 'click');
  assert.equal(ui.storage.has('hbDeploySecret'), false);
  assert.equal(ui.run('getSecret()'), '');
  assert.equal(ui.element('secret').value, '');
  assert.equal(ui.element('secretStatus').textContent, 'Secret cleared.');
  assert.equal(ui.element('deploymentContent').hidden, true);
  assert.equal(ui.element('connectionNotice').hidden, false);
  assert.match(ui.element('connectionNoticeText').textContent, /No HMAC secret/);
  assert.equal(ui.element('setSecret').disabled, true);
  const count = ui.requests.length;
  await ui.run('refreshServerStatus()');
  assert.equal(ui.requests.length, count);
});

test('storage failure leaves the active secret unchanged and prevents reload', async () => {
  const ui = setup({blocked: true});
  ui.type('test-secret');
  await ui.fire('setSecret', 'click');
  assert.equal(ui.run('getSecret()'), '');
  assert.equal(ui.requests.length, 1);
  assert.equal(ui.reloads(), 0);
  assert.equal(ui.storage.has('hbDeployBaseUrl'), false);
  assert.equal(ui.element('secretStatus').textContent, 'Could not save connection in this browser.');
});

test('a failed secret write restores the previous saved API address', async () => {
  const ui = setup({saved: 'previous-secret', savedBase: 'http://localhost:9090', blocked: true});
  ui.element('baseUrl').value = 'http://127.0.0.1:9091';
  ui.type('test-secret');
  await ui.fire('setSecret', 'click');
  assert.equal(ui.storage.get('hbDeployBaseUrl'), 'http://localhost:9090');
  assert.equal(ui.storage.get('hbDeploySecret'), 'previous-secret');
  assert.equal(ui.reloads(), 0);
});

test('a rejected secret keeps the panel open and the previous secret intact without reload', async () => {
  const ui = setup({saved: 'previous-secret', apiError: 'invalid signature'});
  ui.fire('menuToggle', 'click');
  ui.type('test-secret');
  ui.fire('setSecret', 'click');
  await flush();
  assert.equal(ui.element('deployFeedback').hidden, false);
  assert.equal(ui.element('deployFeedbackText').textContent, 'invalid signature');
  assert.equal(ui.element('secretStatus').textContent, 'invalid signature');
  assert.equal(ui.element('menuPanel').classList.contains('hidden'), false);
  assert.equal(ui.element('setSecret').disabled, false);
  assert.equal(ui.run('getSecret()'), 'previous-secret');
  assert.equal(ui.storage.get('hbDeploySecret'), 'previous-secret');
  assert.equal(ui.reloads(), 0);
});

test('Set waits for API acceptance and ignores duplicate submissions before reloading once', async () => {
  let accept;
  const ui = setup({responseGate: new Promise(resolve => { accept = resolve; })});
  ui.type('test-secret');
  const pending = ui.fire('setSecret', 'click');
  await flush();
  assert.equal(ui.element('secretStatus').textContent, 'Checking connection...');
  assert.equal(ui.element('setSecret').disabled, true);
  assert.equal(ui.element('secret').disabled, true);
  assert.equal(ui.element('baseUrl').disabled, true);
  assert.equal(ui.element('clearSecret').disabled, true);
  assert.equal(ui.reloads(), 0);
  assert.equal(ui.storage.has('hbDeploySecret'), false);
  assert.equal(ui.run('getSecret()'), '');
  await ui.fire('setSecret', 'click');
  assert.equal(ui.requests.length, 1);
  accept();
  await pending;
  assert.equal(ui.reloads(), 1);
  assert.equal(ui.storage.get('hbDeploySecret'), 'test-secret');
});

test('an accepted connection preserves the chosen API address across the reload', async () => {
  const ui = setup();
  ui.element('baseUrl').value = 'http://127.0.0.1:9091';
  ui.type('test-secret');
  await ui.fire('setSecret', 'click');
  assert.equal(new URL(ui.requests[0].url).origin, 'http://127.0.0.1:9091');
  assert.equal(ui.reloads(), 1);
  const restored = setup({saved: ui.storage.get('hbDeploySecret'), savedBase: ui.storage.get('hbDeployBaseUrl')});
  assert.equal(restored.element('baseUrl').value, 'http://127.0.0.1:9091');
  assert.equal(restored.run('getSecret()'), 'test-secret');
  await flush();
});

test('Close and Escape dismiss the panel and restore settings-button focus in both modes', async () => {
  for (const mode of ['remote', 'local']) {
    const ui = setup({mode});
    await flush();
    if (mode === 'local') {
      assert.equal(new URL(ui.requests.at(-1).url).pathname, '/local/status');
      assert.equal(ui.requests.at(-1).headers['X-HB-Signature'], undefined);
    }
    ui.fire('menuToggle', 'click');
    assert.equal(ui.element('menuPanel').classList.contains('hidden'), false);
    ui.fire('closeConnection', 'click');
    assert.equal(ui.element('menuPanel').classList.contains('hidden'), true);
    assert.equal(ui.element('menuToggle').attrs.get('aria-expanded'), 'false');
    assert.equal(ui.focused(), 'menuToggle');
    ui.fire('menuToggle', 'click');
    ui.events.get('keydown')({key: 'Escape', preventDefault() {}});
    assert.equal(ui.element('menuPanel').classList.contains('hidden'), true);
    assert.equal(ui.focused(), 'menuToggle');
  }
});

test('without a secret only the connection notice is shown and its action opens settings', async () => {
  const ui = setup({savedView: 'plugins'});
  await flush();
  assert.equal(ui.element('deploymentContent').hidden, true);
  assert.equal(ui.element('connectionNotice').hidden, false);
  assert.match(ui.element('connectionNoticeText').textContent, /deploy\.remote\.hmac_secret/);
  assert.match(ui.element('connectionNoticeText').textContent, /secret itself/);
  assert.equal(ui.element('killAllProcesses').hidden, true);
  assert.equal(ui.element('loadModules').disabled, true);
  assert.equal(ui.requests.length, 0);
  assert.equal(ui.moduleLoads(), 0);
  assert.equal(ui.pluginLoads(), 0);
  ui.fire('openConnection', 'click');
  assert.equal(ui.element('menuPanel').classList.contains('hidden'), false);
  assert.equal(ui.focused(), 'secret');
});

test('a saved secret must be accepted before showing the dashboard or loading any data', async () => {
  let accept;
  const ui = setup({saved: 'test-secret', savedView: 'plugins', responseGate: new Promise(resolve => { accept = resolve; })});
  await flush();
  assert.equal(ui.element('deploymentContent').hidden, true);
  assert.equal(ui.element('connectionNoticeTitle').textContent, 'Checking connection');
  assert.equal(ui.requests.length, 1);
  assert.equal(new URL(ui.requests[0].url).pathname, '/deploy/status');
  assert.equal(ui.moduleLoads(), 0);
  assert.equal(ui.pluginLoads(), 0);
  accept();
  await flush();
  assert.equal(ui.element('deploymentContent').hidden, false);
  assert.equal(ui.element('connectionNotice').hidden, true);
  assert.equal(ui.element('killAllProcesses').hidden, false);
  assert.ok(ui.moduleLoads() > 0);
  assert.equal(ui.pluginLoads(), 1);
});

test('a rejected saved secret shows recovery help, permits retry, and never loads data', async () => {
  const ui = setup({saved: 'wrong-secret', savedView: 'plugins', apiError: 'invalid signature'});
  await flush();
  assert.equal(ui.element('deploymentContent').hidden, true);
  assert.equal(ui.element('connectionNoticeTitle').textContent, 'Secret not accepted');
  assert.equal(ui.element('setSecret').disabled, false);
  assert.equal(ui.moduleLoads(), 0);
  assert.equal(ui.pluginLoads(), 0);
  ui.reply.status = 200;
  ui.reply.error = '';
  await ui.fire('setSecret', 'click');
  assert.equal(ui.reloads(), 1);
});

test('an unreachable API is distinguished from a rejected secret and can recover', async () => {
  const ui = setup({saved: 'test-secret', networkError: 'Failed to fetch'});
  await flush();
  assert.equal(ui.element('deploymentContent').hidden, true);
  assert.equal(ui.element('connectionNoticeTitle').textContent, 'Deployment API unavailable');
  assert.equal(ui.moduleLoads(), 0);
  ui.reply.networkError = '';
  await ui.run('refreshServerStatus()');
  assert.equal(ui.element('deploymentContent').hidden, false);
  assert.equal(ui.moduleLoads(), 1);
});

test('revoked authentication hides an open dashboard, but a non-auth 403 does not', async () => {
  const ui = setup({saved: 'test-secret'});
  await flush();
  assert.equal(ui.element('deploymentContent').hidden, false);
  ui.reply.status = 403;
  ui.reply.error = 'logs disabled';
  await assert.rejects(ui.run('apiRequest("GET", "/deploy/modules/example/logs")'), /logs disabled/);
  assert.equal(ui.element('deploymentContent').hidden, false);
  ui.reply.status = 401;
  ui.reply.error = 'invalid signature';
  await assert.rejects(ui.run('apiRequest("GET", "/deploy/modules")'), /invalid signature/);
  assert.equal(ui.element('deploymentContent').hidden, true);
  assert.equal(ui.element('connectionNoticeTitle').textContent, 'Secret not accepted');
  assert.equal(ui.element('killAllProcesses').hidden, true);
  ui.run('setView("plugins")');
  assert.equal(ui.element('deploymentContent').hidden, true);
  assert.equal(ui.pluginLoads(), 0);
});

test('a late successful check cannot reopen the dashboard after Clear Secret', async () => {
  let accept;
  const ui = setup({saved: 'test-secret', responseGate: new Promise(resolve => { accept = resolve; })});
  await flush();
  ui.fire('clearSecret', 'click');
  accept();
  await flush();
  assert.equal(ui.element('deploymentContent').hidden, true);
  assert.equal(ui.element('connectionNoticeTitle').textContent, 'Connect to the deployment API');
  assert.equal(ui.moduleLoads(), 0);
});

test('local deployment remains available without an HMAC secret', async () => {
  const ui = setup({mode: 'local'});
  assert.equal(ui.element('deploymentContent').hidden, false);
  assert.equal(ui.element('connectionNotice').hidden, true);
  assert.equal(ui.moduleLoads(), 1);
  await flush();
  assert.equal(ui.requests.at(-1).headers['X-HB-Signature'], undefined);
});

test('module cards use explicit compact build labels instead of dash placeholders', () => {
  const ui = setup();
  assert.equal(ui.run('moduleBuildLabel(null)'), 'Select to view builds');
  assert.equal(ui.run('moduleBuildLabel(0)'), 'No packaged builds');
  assert.equal(ui.run('moduleBuildLabel(1)'), '1 packaged build');
  assert.equal(ui.run('moduleBuildLabel(12)'), '12 packaged builds');
});

test('archive filename parsing extracts module and build ID from the canonical name', () => {
  const ui = setup();
  const hash = 'a'.repeat(64);
  assert.equal(
    ui.run(`JSON.stringify(archiveIdentityFromName("hyperbricks-patterns-yaml-1.0-${hash}.hra"))`),
    JSON.stringify({module: 'hyperbricks-patterns-yaml', buildID: hash})
  );
  assert.equal(
    ui.run(`JSON.stringify(archiveIdentityFromName("project-2-demo-1.0-beta.2-${hash.toUpperCase()}.hra"))`),
    JSON.stringify({module: 'project-2-demo', buildID: hash})
  );
  assert.equal(ui.run('archiveIdentityFromName("demo-1.0-manual.hra")'), null);
  assert.equal(ui.run('archiveIdentityFromName("demo.zip")'), null);
});

test('choosing an archive populates read-only upload identity fields from its filename', () => {
  const ui = setup({saved: 'test-secret'});
  const hash = 'b'.repeat(64);
  ui.element('uploadFile').files = [{name: `hyperbricks-patterns-yaml-1.0-${hash}.hra`}];
  ui.fire('uploadFile', 'change');
  assert.equal(ui.element('uploadModule').value, 'hyperbricks-patterns-yaml');
  assert.equal(ui.element('uploadBuildID').value, hash);
  assert.equal(ui.element('uploadError').textContent, '');
});

test('archive upload signs binary bytes with build metadata and content hash', async () => {
  const ui = setup({saved: 'test-secret'});
  await flush();
  ui.requests.length = 0;
  await ui.run('uploadArchiveBytes("demo", "build-123", new Uint8Array([0, 1, 2, 255]))');
  const request = ui.requests.at(-1);
  const headers = request.headers;
  const bodyHash = createHash('sha256').update(Buffer.from([0, 1, 2, 255])).digest('hex');
  const canonical = ['POST', '/deploy/v1/modules/demo/releases', bodyHash, headers['X-HB-Timestamp'], headers['X-HB-Nonce'], '', 'build-123'].join('\n');
  assert.equal(new URL(request.url).pathname, '/deploy/v1/modules/demo/releases');
  assert.equal(headers['Content-Type'], 'application/vnd.hyperbricks.hra');
  assert.equal(headers['X-HB-Build-ID'], 'build-123');
  assert.equal(headers['X-HB-SHA256'], bodyHash);
  assert.equal(headers['X-HB-Signature'], createHmac('sha256', 'test-secret').update(canonical).digest('hex'));
  assert.deepEqual(Array.from(request.body), [0, 1, 2, 255]);
});

test('archive upload rejects local mode and invalid identifiers before fetching', async () => {
  const local = setup({mode: 'local'});
  const remote = setup({saved: 'test-secret'});
  await flush();
  const remoteCount = remote.requests.length;
  await assert.rejects(local.run('uploadArchiveBytes("demo", "build", new Uint8Array([1]))'), /remote deployment dashboard/);
  await assert.rejects(remote.run('uploadArchiveBytes("../demo", "build", new Uint8Array([1]))'), /valid module name/);
  await assert.rejects(remote.run('uploadArchiveBytes("demo", "../build", new Uint8Array([1]))'), /valid build ID/);
  assert.equal(remote.requests.length, remoteCount);
});
