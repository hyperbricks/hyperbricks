import {readFileSync} from 'node:fs';
import {createHash, createHmac, randomFillSync} from 'node:crypto';
import vm from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';

const html = readFileSync(new URL('./deploy_dashboard.html', import.meta.url), 'utf8');
const source = html.match(/<script>([\s\S]*?)<\/script>/)[1];
const flush = () => new Promise(resolve => setImmediate(resolve));

function setup({
  saved = '',
  savedBase = '',
  savedView = '',
  blocked = false,
  mode = 'remote',
  apiError = '',
  networkError = '',
  responseGate,
  packageConfigGate,
  packageConfig,
  editorModule,
  editorImportError = '',
  confirmResult = true
} = {}) {
  const elements = new Map(), events = new Map(), requests = [];
  const editorImportURLs = [];
  const storage = new Map(saved ? [['hbDeploySecret', saved]] : []);
  if (savedBase) storage.set('hbDeployBaseUrl', savedBase);
  if (savedView) storage.set('hbDeployView', savedView);
  const reply = {status: apiError ? 401 : 200, error: apiError, networkError};
  let focused = '', reloads = 0, moduleLoads = 0, pluginLoads = 0, editorImports = 0;
  for (const [, id] of html.matchAll(/\bid="([^"]+)"/g)) {
    const classes = new Set(id === 'menuPanel' ? ['hidden'] : []);
    const listeners = new Map(), attrs = new Map();
    let currentValue = '';
    const element = {
      textContent: '', disabled: false, hidden: false, open: false, files: [], listeners, attrs,
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
    };
    Object.defineProperty(element, 'value', {
      get: () => currentValue,
      set: value => {
        const source = String(value ?? '');
        currentValue = id === 'packageConfigContent' ? source.replace(/\r\n?/g, '\n') : source;
      }
    });
    elements.set(id, element);
  }
  const context = vm.createContext({
    document: {
      body: {dataset: {mode}, classList: {add() {}}},
      getElementById: id => elements.get(id),
      querySelectorAll: () => [],
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
    confirm: () => confirmResult,
    fetch: async (url, options = {}) => {
      requests.push({url, ...options});
      if (responseGate) await responseGate;
      if (reply.networkError) throw Error(reply.networkError);
      const path = new URL(url).pathname;
      if (packageConfigGate && path.endsWith('/package-config')) await packageConfigGate;
      let payload = {version: 'test'};
      if (packageConfig && path.endsWith('/package-config')) {
        if ((options.method || 'GET') === 'PUT') {
          const submitted = JSON.parse(options.body);
          payload = {...packageConfig, content: submitted.content, sha256: 'saved-sha256'};
        } else {
          payload = packageConfig;
        }
      }
      return {ok: reply.status === 200, status: reply.status, json: async () => reply.error ? {error: reply.error} : payload};
    }
  });
  // Keep production event wiring and initialization, but avoid unrelated module rendering.
  vm.runInContext(source.replace(/\s+wireEvents\(\);\s+initDefaults\(\);\s*$/, ''), context);
  if (editorModule || editorImportError) {
    context.testEditorImporter = (retrySuffix = '') => {
      editorImports++;
      editorImportURLs.push('/assets/deploy-yaml-editor.js' + retrySuffix);
      return editorImportError ? Promise.reject(new Error(editorImportError)) : Promise.resolve(editorModule);
    };
    vm.runInContext('packageConfigEditorImporter = testEditorImporter;', context);
  }
  vm.runInContext('loadModules = recordModuleLoad; loadGlobalPlugins = recordPluginLoad; renderPluginModules = () => {}; renderPluginBuilds = () => {}; wireEvents(); initDefaults();', context);
  const element = id => elements.get(id);
  const fire = (id, name, event = {}) => element(id).listeners.get(name)({preventDefault() {}, stopPropagation() {}, ...event});
  const type = value => { element('secret').value = value; fire('secret', 'input'); };
  return {
    element,
    fire,
    type,
    events,
    storage,
    requests,
    reply,
    moduleLoads: () => moduleLoads,
    pluginLoads: () => pluginLoads,
    reloads: () => reloads,
    editorImports: () => editorImports,
    editorImportURLs: () => [...editorImportURLs],
    focused: () => focused,
    expose: (name, value) => { context[name] = value; },
    run: code => vm.runInContext(code, context)
  };
}

function createEditorHarness() {
  const instances = [];
  const module = {
    createDeployYAMLEditor(options) {
      let value = String(options.doc ?? '');
      const instance = {
        options,
        readOnly: Boolean(options.readOnly),
        focused: 0,
        destroyed: 0,
        getValue: () => value,
        setValue(next) {
          value = String(next ?? '');
          options.onChange(value);
        },
        setReadOnly(next) { this.readOnly = Boolean(next); },
        focus() { this.focused++; },
        destroy() { this.destroyed++; },
        userChange(next) {
          value = String(next);
          options.onChange(value);
        },
        save() { options.onSave(); },
        validate(result) { options.onValidation(result); }
      };
      instances.push(instance);
      options.onValidation({valid: true});
      return instance;
    }
  };
  return {module, instances, current: () => instances.at(-1)};
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

test('runtime modes use explicit Development and Live labels with legacy production compatibility', () => {
  const ui = setup();
  assert.equal(ui.run('buildRuntimeMode({runtime_mode: "development", production: true})'), 'development');
  assert.equal(ui.run('buildRuntimeMode({runtime_mode: "live", production: false})'), 'live');
  assert.equal(ui.run('buildRuntimeMode({production: true})'), 'live');
  assert.equal(ui.run('buildRuntimeMode({production: false})'), 'development');
  assert.equal(ui.run('buildRuntimeMode({runtime_mode: "unknown", production: true})'), 'live');
  assert.equal(ui.run('runtimeModeLabel("development")'), 'Development');
  assert.equal(ui.run('runtimeModeLabel("live")'), 'Live');
});

test('mode updates use the explicit signed contract and restore the selector after failure', async () => {
  const ui = setup({saved: 'test-secret'});
  await flush();
  ui.requests.length = 0;
  ui.run(`
    renderBuilds = () => {};
    state.selectedModule = "demo";
    state.selectionToken = 7;
    state.builds = [{build_id: "build-1", runtime_mode: "development", production: false}];
    testModeSelect = {value: "live", disabled: false, isConnected: true};
  `);

  await ui.run('setBuildMode("build-1", "live", testModeSelect)');
  const request = ui.requests.at(-1);
  const headers = request.headers;
  const bodyHash = createHash('sha256').update(request.body).digest('hex');
  const canonical = ['PUT', '/deploy/modules/demo/builds/build-1/mode', bodyHash, headers['X-HB-Timestamp'], headers['X-HB-Nonce']].join('\n');
  assert.equal(new URL(request.url).pathname, '/deploy/modules/demo/builds/build-1/mode');
  assert.equal(request.method, 'PUT');
  assert.equal(request.body, JSON.stringify({mode: 'live'}));
  assert.equal(headers['X-HB-Signature'], createHmac('sha256', 'test-secret').update(canonical).digest('hex'));
  assert.equal(ui.run('state.builds[0].runtime_mode'), 'live');
  assert.equal(ui.run('testModeSelect.disabled'), false);

  ui.reply.status = 500;
  ui.reply.error = 'mode rejected';
  ui.run('state.builds[0].runtime_mode = "development"; state.builds[0].production = false; testModeSelect.value = "live";');
  await assert.rejects(ui.run('setBuildMode("build-1", "live", testModeSelect)'), /mode rejected/);
  assert.equal(ui.run('testModeSelect.value'), 'development');
  assert.equal(ui.run('testModeSelect.disabled'), false);
});

test('package configuration routes share the local and remote API prefixes and encode identifiers', () => {
  const remote = setup();
  const local = setup({mode: 'local'});
  assert.equal(
    remote.run('packageConfigPath("owner/demo", "build id")'),
    '/deploy/modules/owner%2Fdemo/builds/build%20id/package-config'
  );
  assert.equal(
    local.run('packageConfigPath("owner/demo", "build id")'),
    '/local/modules/owner%2Fdemo/builds/build%20id/package-config'
  );
});

test('package YAML editor loads lazily once and saves exact raw text', async () => {
  const source = '# keep this comment\r\nhyperbricks:\r\n  mode: "live"\r\n\r\nvars:\r\n  free_value: "001"\r\n';
  const edited = source.replace('"001"', '"002"');
  const editor = createEditorHarness();
  const ui = setup({
    mode: 'local',
    editorModule: editor.module,
    packageConfig: {content: source, sha256: 'opened-sha256', scope: 'runtime', restart_required: false}
  });
  await flush();
  assert.equal(ui.editorImports(), 0);

  ui.run('state.selectedModule = "demo"; state.selectionToken = 11;');
  await ui.run('openPackageConfig("build-1", null)');
  await flush();
  assert.equal(ui.editorImports(), 1);
  assert.equal(editor.instances.length, 1);
  assert.equal(editor.current().getValue(), source);
  assert.equal(ui.element('packageConfigContent').value, source.replaceAll('\r\n', '\n'));
  assert.equal(ui.element('packageConfigContent').hidden, true);
  assert.equal(ui.element('savePackageConfig').disabled, true);

  editor.current().userChange(edited);
  assert.equal(ui.element('savePackageConfig').disabled, false);
  editor.current().validate({valid: false, line: 3, column: 9, message: 'unexpected value'});
  assert.match(ui.element('packageConfigEditorStatus').textContent, /Line 3, column 9: unexpected value/);
  assert.equal(ui.element('savePackageConfig').disabled, false, 'client syntax feedback must not replace Go validation');

  editor.current().save();
  await flush();
  const put = ui.requests.find(request => request.method === 'PUT' && new URL(request.url).pathname.endsWith('/package-config'));
  assert.ok(put);
  assert.deepEqual(JSON.parse(put.body), {content: edited, expected_sha256: 'opened-sha256'});
  assert.equal(editor.current().readOnly, false);
  assert.equal(ui.element('savePackageConfig').disabled, true);

  assert.equal(ui.run('closePackageConfigDialog(true)'), true);
  assert.equal(editor.instances[0].destroyed, 1);
  ui.run('state.selectedModule = "demo"; state.selectionToken = 12;');
  await ui.run('openPackageConfig("build-2", null)');
  await flush();
  assert.equal(ui.editorImports(), 1, 'the already imported bundle should be reused');
  assert.equal(editor.instances.length, 2, 'each dialog session gets fresh editor state and undo history');
});

test('failed YAML editor import keeps a retryable exact-text fallback', async () => {
  const source = '# fallback\r\nhyperbricks:\r\n  mode: development\r\n';
  const edited = source + 'free: "yes"\r\n';
  const ui = setup({
    mode: 'local',
    editorImportError: 'asset unavailable',
    packageConfig: {content: source, sha256: 'fallback-sha', scope: 'source', restart_required: false}
  });
  await flush();
  ui.run('state.selectedModule = "demo"; state.selectionToken = 21;');
  await ui.run('openPackageConfig("dev", null)');
  await flush();

  assert.equal(ui.element('packageConfigContent').hidden, false);
  assert.equal(ui.element('packageConfigContent').disabled, false);
  assert.equal(ui.element('packageConfigContent').value, source.replaceAll('\r\n', '\n'));
  assert.equal(ui.run('getPackageConfigValue()'), source);
  assert.equal(ui.element('savePackageConfig').disabled, true, 'textarea normalization alone must not make CRLF input dirty');
  assert.match(ui.element('packageConfigEditorStatus').textContent, /plain text editor remains available/);
  assert.deepEqual(ui.editorImportURLs(), ['/assets/deploy-yaml-editor.js']);

  ui.element('packageConfigContent').value = edited;
  ui.fire('packageConfigContent', 'input');
  let prevented = 0;
  ui.fire('packageConfigContent', 'keydown', {key: 's', ctrlKey: true, preventDefault: () => { prevented++; }});
  await flush();
  assert.equal(prevented, 1);
  const put = ui.requests.find(request => request.method === 'PUT' && new URL(request.url).pathname.endsWith('/package-config'));
  assert.equal(JSON.parse(put.body).content, edited);

  assert.equal(ui.run('closePackageConfigDialog(true)'), true);
  const retry = createEditorHarness();
  let retryImports = 0;
  let retrySuffix = '';
  ui.expose('retryEditorImporter', suffix => {
    retryImports++;
    retrySuffix = suffix;
    return Promise.resolve(retry.module);
  });
  ui.run('packageConfigEditorImporter = retryEditorImporter; state.selectedModule = "demo"; state.selectionToken = 22;');
  await ui.run('openPackageConfig("dev", null)');
  await flush();
  assert.equal(retryImports, 1, 'a rejected import promise must not poison later attempts');
  assert.equal(retrySuffix, '?retry=1', 'retry must bypass the browser module map failure cache');
  assert.equal(retry.instances.length, 1);
});

test('typing in the fallback is retained while a slow YAML editor import finishes', async () => {
  let resolveEditor;
  const pendingEditor = new Promise(resolve => { resolveEditor = resolve; });
  const editor = createEditorHarness();
  const source = 'hyperbricks:\n  mode: development\n';
  const edited = source + 'free_variable: "kept"\n';
  const ui = setup({
    mode: 'local',
    editorModule: pendingEditor,
    packageConfig: {content: source, sha256: 'sha', scope: 'source', restart_required: false}
  });
  await flush();
  ui.run('state.selectedModule = "demo"; state.selectionToken = 30;');
  await ui.run('openPackageConfig("dev", null)');
  assert.equal(ui.element('packageConfigContent').hidden, false);

  ui.element('packageConfigContent').value = edited;
  ui.fire('packageConfigContent', 'input');
  resolveEditor(editor.module);
  await flush();

  assert.equal(editor.instances.length, 1);
  assert.equal(editor.current().getValue(), edited);
  assert.equal(ui.element('packageConfigContent').value, edited);
  assert.equal(ui.element('savePackageConfig').disabled, false);
});

test('a late YAML editor import cannot mount into a closed configuration dialog', async () => {
  let resolveEditor;
  const pendingEditor = new Promise(resolve => { resolveEditor = resolve; });
  const editor = createEditorHarness();
  const ui = setup({
    mode: 'local',
    editorModule: pendingEditor,
    packageConfig: {content: 'hyperbricks:\n  mode: live\n', sha256: 'sha', scope: 'runtime', restart_required: false}
  });
  await flush();
  ui.run('state.selectedModule = "demo"; state.selectionToken = 31;');
  await ui.run('openPackageConfig("build-1", null)');
  assert.equal(ui.run('closePackageConfigDialog(true)'), true);
  resolveEditor(editor.module);
  await flush();
  assert.equal(editor.instances.length, 0);
  assert.equal(ui.element('packageConfigEditorHost').hidden, true);
});

test('a pending package configuration load can be cancelled and its late response is ignored', async () => {
  let releaseConfig;
  const packageConfigGate = new Promise(resolve => { releaseConfig = resolve; });
  const editor = createEditorHarness();
  const ui = setup({
    mode: 'local',
    editorModule: editor.module,
    packageConfigGate,
    packageConfig: {content: 'hyperbricks:\n  mode: live\n', sha256: 'sha', scope: 'runtime', restart_required: false}
  });
  await flush();
  ui.run('state.selectedModule = "demo"; state.selectionToken = 32;');
  const opening = ui.run('openPackageConfig("build-1", null)');
  await flush();

  assert.equal(ui.element('packageConfigDialog').open, true);
  assert.equal(ui.element('packageConfigContent').disabled, true);
  assert.equal(ui.element('cancelPackageConfig').disabled, false);
  ui.fire('packageConfigDialog', 'cancel');
  assert.equal(ui.element('packageConfigDialog').open, false);

  releaseConfig();
  await opening;
  await flush();
  assert.equal(editor.instances.length, 0);
  assert.equal(ui.run('state.packageConfigBuild'), '');
  assert.equal(ui.run('state.packageConfigBusy'), false);
});

test('archive filenames prefer RFC 5987, support quoted names, and fall back safely', () => {
  const ui = setup();
  assert.equal(
    ui.run(`filenameFromDisposition("attachment; filename*=UTF-8''demo%20release.hra; filename=ignored.hra", "fallback.hra")`),
    'demo release.hra'
  );
  assert.equal(
    ui.run('filenameFromDisposition(\'attachment; filename="demo-build.hra"\', "fallback.hra")'),
    'demo-build.hra'
  );
  assert.equal(
    ui.run('filenameFromDisposition("attachment; filename=demo-build.hra", "fallback.hra")'),
    'demo-build.hra'
  );
  assert.equal(
    ui.run(`filenameFromDisposition("attachment; filename*=UTF-8''%E0%A4%A", "fallback.hra")`),
    'fallback.hra'
  );
  assert.equal(ui.run('filenameFromDisposition("attachment", "fallback.hra")'), 'fallback.hra');
});

test('deployment markup exposes mode, package editor, and archive-download contracts', () => {
  for (const value of [
    'id="packageConfigDialog"',
    'id="packageConfigForm"',
    'id="packageConfigEditorShell"',
    'id="packageConfigEditorHost"',
    'id="packageConfigContent"',
    'id="packageConfigEditorStatus"',
    'id="packageConfigError"',
    'id="packageConfigStatus"',
    'id="savePackageConfig"',
    'value="development"',
    '>Development</option>',
    'value="live"',
    '>Live</option>',
    'action: "config"',
    'action: "download"',
    '"/package-config"',
    'import("/assets/deploy-yaml-editor.js")',
    '"/archive"'
  ]) {
    assert.ok(html.includes(value), `deployment markup missing ${value}`);
  }
  assert.doesNotMatch(html, /<script[^>]+src="\/assets\/deploy-yaml-editor\.js"/);
  assert.doesNotMatch(html, />Default<\/option>|>Production<\/option>/);
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
