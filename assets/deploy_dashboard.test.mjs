import {readFileSync} from 'node:fs';
import {createHash, createHmac, randomFillSync} from 'node:crypto';
import vm from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';

const html = readFileSync(new URL('./deploy_dashboard.html', import.meta.url), 'utf8');
const css = readFileSync(new URL('./dashboard.css', import.meta.url), 'utf8');
const source = html.match(/<script>([\s\S]*?)<\/script>/)[1];
const flush = () => new Promise(resolve => setImmediate(resolve));

function setup({
  saved = '',
  savedBase = '',
  savedView = '',
  blocked = false,
  mode = 'remote',
  narrow = false,
  apiError = '',
  networkError = '',
  responseGate,
  packageConfigGate,
  packageConfig,
  credentials,
  credentialsGate,
  modeResponse,
  responseForRequest,
  editorModule,
  editorImportError = '',
  stubModuleLoad = true,
  stubGlobalLoad = true,
  stubPluginBuilds = true,
  confirmResult = true
} = {}) {
  const elements = new Map(), events = new Map(), requests = [];
  const editorImportURLs = [];
  const storage = new Map(saved ? [['hbDeploySecret', saved]] : []);
  if (savedBase) storage.set('hbDeployBaseUrl', savedBase);
  if (savedView) storage.set('hbDeployView', savedView);
  const reply = {status: apiError ? 401 : 200, error: apiError, networkError};
  const moduleMedia = {matches: !narrow, addEventListener(name, fn) { if (name === 'change') this.onchange = fn; }};
  let focused = '', reloads = 0, moduleLoads = 0, pluginLoads = 0, editorImports = 0;
  const makeNode = (tagName) => {
    const attrs = new Map(), listeners = new Map(), classes = new Set();
    const node = {
      tagName, attrs, listeners, children: [], dataset: {}, disabled: false,
      textContent: '',
      setAttribute: (name, value) => attrs.set(name, value),
      addEventListener: (name, fn) => listeners.set(name, fn),
      appendChild(child) { this.children.push(child); return child; },
      append(...children) { this.children.push(...children); },
      querySelectorAll: () => [],
      focus: () => { focused = tagName; },
      classList: {
        add: name => classes.add(name),
        remove: name => classes.delete(name),
        contains: name => classes.has(name),
        toggle: (name, force = !classes.has(name)) => force ? classes.add(name) : classes.delete(name)
      }
    };
    let innerHTML = '';
    Object.defineProperty(node, 'innerHTML', {
      get: () => innerHTML,
      set: value => { innerHTML = String(value); node.children = []; }
    });
    return node;
  };
  for (const [, id] of html.matchAll(/\bid="([^"]+)"/g)) {
    const classes = new Set(id === 'menuPanel' ? ['hidden'] : []);
    const listeners = new Map(), attrs = new Map();
    let currentValue = '';
    const element = {
      textContent: '', disabled: false, hidden: false, open: false, checked: false, files: [], listeners, attrs,
      addEventListener: (name, fn) => listeners.set(name, fn),
      setAttribute: (name, value) => attrs.set(name, value),
      removeAttribute: name => attrs.delete(name),
      focus: () => { focused = id; },
      reset: () => {},
      showModal() { this.open = true; },
      close() { this.open = false; },
      querySelector: () => ({textContent: ''}),
      querySelectorAll: (selector) => selector === 'tr[data-plugin-key]'
        ? element.children.filter(child => child.tagName === 'tr' && child.dataset.pluginKey)
        : [],
      children: [],
      appendChild(child) { this.children.push(child); return child; },
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
    let innerHTML = '';
    Object.defineProperty(element, 'innerHTML', {
      configurable: true,
      get: () => innerHTML,
      set: value => { innerHTML = String(value); element.children = []; }
    });
    elements.set(id, element);
  }
  const context = vm.createContext({
    document: {
      body: {dataset: {mode}, classList: {add() {}}},
      getElementById: id => elements.get(id),
      createElement: makeNode,
      querySelectorAll: () => [],
      addEventListener: (name, fn) => events.set(name, fn)
    },
    window: {
      location: {origin: 'http://localhost:9090', reload: () => { reloads++; }},
      matchMedia: () => moduleMedia,
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
    URL, URLSearchParams, TextEncoder, Uint8Array, ArrayBuffer, crypto: {getRandomValues: bytes => randomFillSync(bytes)}, setInterval: () => 1,
    setTimeout: callback => { callback(); return 1; },
    recordModuleLoad: async () => { moduleLoads++; },
    recordPluginLoad: async () => { pluginLoads++; },
    confirm: () => confirmResult,
    fetch: async (url, options = {}) => {
      requests.push({url, ...options});
      if (responseGate) await responseGate;
      if (reply.networkError) throw Error(reply.networkError);
      const path = new URL(url).pathname;
      if (responseForRequest) {
        const fixture = await responseForRequest(new URL(url), options, requests.length);
        if (fixture !== undefined) {
          const status = fixture.status || 200;
          return {ok: status >= 200 && status < 300, status, json: async () => fixture};
        }
      }
      if (packageConfigGate && path.endsWith('/package-config')) await packageConfigGate;
      if (credentialsGate && path.endsWith('/credentials')) await credentialsGate;
      let payload = {version: 'test'};
      if (credentials && path.endsWith('/credentials')) {
        payload = credentials;
        if ((options.method || 'GET') === 'PUT') {
          const submitted = JSON.parse(options.body);
          payload = {...credentials, sha256: 'saved-credentials-sha'};
          for (const field of ['user', 'password']) {
            if (submitted[field] !== undefined) payload[field] = {source: 'literal', value: submitted[field]};
          }
          for (const field of ['dashboard_enabled', 'spaces_enabled']) {
            if (submitted[field] !== undefined) payload[field] = submitted[field];
          }
          credentials = payload;
        }
      }
      if (modeResponse && path.endsWith('/mode')) payload = modeResponse;
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
  vm.runInContext(`${stubModuleLoad ? 'loadModules = recordModuleLoad;' : ''} ${stubGlobalLoad ? 'loadGlobalPlugins = recordPluginLoad;' : ''} if (typeof renderPluginModules === "function") renderPluginModules = () => {}; ${stubPluginBuilds ? 'renderPluginBuilds = () => {};' : ''} wireEvents(); initDefaults();`, context);
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
    setNarrow: (isNarrow) => { moduleMedia.matches = !isNarrow; moduleMedia.onchange?.(); },
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

const credentialFixture = {
  sha256: 'opened-credentials-sha', scope: 'runtime', restart_required: true,
  user: {source: 'literal', value: 'demo-user'},
  password: {source: 'literal', value: 'demo-password'}
};

test('credentials load masked, reveal explicitly, and clear completely on close in both modes', async () => {
  for (const mode of ['local', 'remote']) {
    const ui = setup({mode, saved: 'test-secret', credentials: credentialFixture});
    await flush();
    ui.run('state.selectedModule = "demo"');
    await ui.run('openCredentials("build-1", ui.refreshStatus)');
    assert.equal(ui.element('credentialsDialog').open, true);
    assert.equal(ui.element('credentialsUser').value, 'demo-user');
    assert.equal(ui.element('credentialsPassword').type, 'password');
    assert.equal(ui.element('credentialsPassword').value, 'demo-password');
    assert.equal(ui.element('saveCredentials').disabled, true);
    assert.match(ui.requests.at(-1).url, /\/modules\/demo\/builds\/build-1\/credentials$/);
    if (mode === 'remote') assertSignedWith(ui.requests.at(-1), 'test-secret');
    ui.fire('showCredentials', 'click');
    assert.equal(ui.element('credentialsPassword').type, 'text');
    assert.equal(ui.element('showCredentials').attrs.get('aria-pressed'), 'true');
    ui.fire('cancelCredentials', 'click');
    assert.equal(ui.element('credentialsDialog').open, false);
    for (const field of ['credentialsUser', 'credentialsPassword', 'credentialsNewUser', 'credentialsNewPassword']) {
      assert.equal(ui.element(field).value, '');
    }
    assert.equal(ui.element('credentialsPassword').type, 'password');
    assert.equal(ui.focused(), 'refreshStatus');
    assert.doesNotMatch(JSON.stringify([...ui.storage]), /demo-password|demo-user/);
  }
});

test('developer access loads explicit feature flags and defaults in both deployment modes', async () => {
  assert.match(html, /id="credentialsTitle" tabindex="-1">Developer access<\/h2>/);
  for (const mode of ['local', 'remote']) {
    for (const flags of [{}, {dashboard_enabled: true, spaces_enabled: false}, {dashboard_enabled: false, spaces_enabled: true}]) {
      const ui = setup({mode, saved: 'test-secret', credentials: {...credentialFixture, ...flags}});
      await flush();
      ui.run('state.selectedModule = "demo"');
      await ui.run('openCredentials("build-1")');
      assert.equal(ui.focused(), 'credentialsTitle');
      assert.equal(ui.element('credentialsDashboardEnabled').checked, flags.dashboard_enabled ?? false);
      assert.equal(ui.element('credentialsSpacesEnabled').checked, flags.spaces_enabled ?? true);
      assert.equal(ui.element('credentialsDashboardEnabled').disabled, false);
      assert.equal(ui.element('credentialsSpacesEnabled').disabled, false);
      assert.equal(ui.element('saveCredentials').disabled, true);
      assert.equal(ui.run('credentialsAreDirty()'), false);
    }
  }
});

test('feature toggles alone save only changed booleans with the current hash and reset dirty state', async () => {
  for (const mode of ['local', 'remote']) {
    for (const changes of [{dashboard_enabled: true}, {spaces_enabled: false}, {dashboard_enabled: true, spaces_enabled: false}]) {
      const ui = setup({mode, saved: 'test-secret', credentials: {
        ...credentialFixture,
        user: {source: 'missing'}, password: {source: 'missing'}
      }});
      await flush();
      ui.run('state.selectedModule = "demo"');
      await ui.run('openCredentials("build-1")');
      const before = ui.requests.length;
      await ui.fire('credentialsForm', 'submit');
      assert.equal(ui.requests.length, before, 'an unchanged form must not submit');
      for (const [field, checked] of Object.entries(changes)) {
        const id = field === 'dashboard_enabled' ? 'credentialsDashboardEnabled' : 'credentialsSpacesEnabled';
        ui.element(id).checked = checked;
        ui.fire(id, 'change');
      }
      assert.equal(ui.element('saveCredentials').disabled, false);
      assert.equal(ui.run('credentialsAreDirty()'), true);
      await ui.fire('credentialsForm', 'submit');
      const request = ui.requests.at(-1);
      assert.equal(request.method, 'PUT');
      assert.deepEqual(JSON.parse(request.body), {expected_sha256: 'opened-credentials-sha', ...changes});
      assert.equal(ui.element('credentialsDashboardEnabled').checked, changes.dashboard_enabled ?? false);
      assert.equal(ui.element('credentialsSpacesEnabled').checked, changes.spaces_enabled ?? true);
      assert.equal(ui.element('saveCredentials').disabled, true);
      assert.equal(ui.run('credentialsAreDirty()'), false);
      assert.equal(ui.run('state.credentialsSHA'), 'saved-credentials-sha');
      assert.equal(ui.element('credentialsError').textContent, '');
      const after = ui.requests.length;
      await ui.fire('credentialsForm', 'submit');
      assert.equal(ui.requests.length, after, 'saved toggle values must become the clean baseline');
      ui.element('credentialsDashboardEnabled').checked = !(changes.dashboard_enabled ?? false);
      ui.fire('credentialsDashboardEnabled', 'change');
      await ui.fire('credentialsForm', 'submit');
      assert.deepEqual(JSON.parse(ui.requests.at(-1).body), {
        expected_sha256: 'saved-credentials-sha', dashboard_enabled: !(changes.dashboard_enabled ?? false)
      });
      assert.equal(ui.element('credentialsSpacesEnabled').checked, changes.spaces_enabled ?? true);
      assert.equal(ui.element('saveCredentials').disabled, true);
    }
  }
});

test('reverting a feature toggle restores the clean state without changing credentials', async () => {
  const ui = setup({mode: 'local', credentials: credentialFixture});
  ui.run('state.selectedModule = "demo"');
  await ui.run('openCredentials("build-1")');
  for (const id of ['credentialsDashboardEnabled', 'credentialsSpacesEnabled']) {
    const original = ui.element(id).checked;
    ui.element(id).checked = !original;
    ui.fire(id, 'change');
    assert.equal(ui.element('saveCredentials').disabled, false);
    ui.element(id).checked = original;
    ui.fire(id, 'change');
    assert.equal(ui.element('saveCredentials').disabled, true);
    assert.equal(ui.run('credentialsAreDirty()'), false);
  }
  assert.equal(ui.element('credentialsUser').value, 'demo-user');
  assert.equal(ui.element('credentialsPassword').value, 'demo-password');
});

test('Spaces visibility explains a disabled frontend-editing parent without silently changing it', async () => {
  const ui = setup({mode: 'local', credentials: {...credentialFixture, spaces_enabled: false, frontend_editing_enabled: false}});
  ui.run('state.selectedModule = "demo"');
  await ui.run('openCredentials("build-1")');
  assert.equal(ui.element('credentialsSpacesParentWarning').hidden, true);
  ui.element('credentialsSpacesEnabled').checked = true;
  ui.fire('credentialsSpacesEnabled', 'change');
  assert.equal(ui.element('credentialsSpacesParentWarning').hidden, false);
  await ui.fire('credentialsForm', 'submit');
  assert.deepEqual(JSON.parse(ui.requests.at(-1).body), {expected_sha256: 'opened-credentials-sha', spaces_enabled: true});
  assert.equal(ui.element('credentialsSpacesParentWarning').hidden, false);
  assert.equal(ui.element('saveCredentials').disabled, true);
});

test('feature toggles and credential changes can be saved together without replacing untouched references', async () => {
  const ui = setup({mode: 'local', credentials: {...credentialFixture, user: {source: 'reference', value: 'env: DEMO_USER'}}});
  ui.run('state.selectedModule = "demo"');
  await ui.run('openCredentials("build-1")');
  ui.element('credentialsSpacesEnabled').checked = false;
  ui.fire('credentialsSpacesEnabled', 'change');
  ui.element('credentialsNewPassword').value = 'replacement';
  ui.fire('credentialsNewPassword', 'input');
  await ui.fire('credentialsForm', 'submit');
  assert.deepEqual(JSON.parse(ui.requests.at(-1).body), {
    expected_sha256: 'opened-credentials-sha', password: 'replacement', spaces_enabled: false
  });
  assert.equal(ui.element('credentialsUser').value, 'env: DEMO_USER');
  assert.equal(ui.element('credentialsPassword').value, 'replacement');
  assert.equal(ui.element('credentialsSpacesEnabled').checked, false);
  assert.equal(ui.element('credentialsNewPassword').value, '');
  assert.equal(ui.element('saveCredentials').disabled, true);
});

test('unsaved toggle changes participate in discard confirmation and reset when reopened', async () => {
  for (const confirmResult of [false, true]) {
    for (const id of ['credentialsDashboardEnabled', 'credentialsSpacesEnabled']) {
      const ui = setup({mode: 'local', confirmResult, credentials: credentialFixture});
      ui.run('state.selectedModule = "demo"');
      await ui.run('openCredentials("build-1")');
      const original = ui.element(id).checked;
      ui.element(id).checked = !original;
      ui.fire(id, 'change');
      const before = ui.requests.length;
      ui.fire('cancelCredentials', 'click');
      assert.equal(ui.requests.length, before, 'closing must not save toggle changes');
      assert.equal(ui.element('credentialsDialog').open, !confirmResult);
      if (!confirmResult) {
        assert.equal(ui.element(id).checked, !original);
        assert.equal(ui.element('saveCredentials').disabled, false);
        ui.run('closeCredentialsDialog(true)');
      }
      await ui.run('openCredentials("build-1")');
      assert.equal(ui.element(id).checked, original);
      assert.equal(ui.element('saveCredentials').disabled, true);
      assert.equal(ui.run('credentialsAreDirty()'), false);
    }
  }
});

test('changing only a password preserves username references and uses a hash-protected signed save', async () => {
  const ui = setup({saved: 'test-secret', credentials: {...credentialFixture, user: {source: 'reference', value: 'env: DEMO_USER'}}});
  await flush();
  ui.run('state.selectedModule = "demo"');
  await ui.run('openCredentials("build-1")');
  assert.match(ui.element('credentialsUserSource').textContent, /reference.*not resolved/);
  ui.element('credentialsNewPassword').value = ' new: #secret ';
  ui.fire('credentialsNewPassword', 'input');
  ui.fire('showCredentials', 'click');
  await ui.fire('credentialsForm', 'submit');
  const request = ui.requests.at(-1);
  assert.equal(request.method, 'PUT');
  assert.deepEqual(JSON.parse(request.body), {expected_sha256: 'opened-credentials-sha', password: ' new: #secret '});
  const headers = request.headers;
  const canonical = [request.method, new URL(request.url).pathname, createHash('sha256').update(request.body).digest('hex'), headers['X-HB-Timestamp'], headers['X-HB-Nonce']].join('\n');
  assert.equal(headers['X-HB-Signature'], createHmac('sha256', 'test-secret').update(canonical).digest('hex'));
  assert.equal(ui.element('credentialsPassword').value, ' new: #secret ');
  assert.equal(ui.element('credentialsUser').value, 'env: DEMO_USER');
  assert.equal(ui.element('credentialsPassword').type, 'password');
  assert.equal(ui.element('credentialsNewPassword').value, '');
  assert.match(ui.element('credentialsStatus').textContent, /Restart/);
  assert.doesNotMatch(ui.element('activity').textContent, /secret|DEMO_USER/);
  assert.equal(ui.run('state.credentialsSHA'), 'saved-credentials-sha');
});

test('missing credentials require both fields and can be inserted for source modules', async () => {
  const ui = setup({mode: 'local', credentials: {...credentialFixture, scope: 'source', restart_required: false, user: {source: 'missing'}, password: {source: 'missing'}}});
  ui.run('state.selectedModule = "demo"');
  await ui.run('openCredentials("dev")');
  assert.match(ui.element('credentialsScope').textContent, /source module/);
  ui.element('credentialsNewPassword').value = 'new-password';
  const count = ui.requests.length;
  await ui.fire('credentialsForm', 'submit');
  assert.equal(ui.requests.length, count);
  assert.match(ui.element('credentialsError').textContent, /both/);
  ui.element('credentialsNewUser').value = 'new-user';
  await ui.fire('credentialsForm', 'submit');
  assert.deepEqual(JSON.parse(ui.requests.at(-1).body), {expected_sha256: 'opened-credentials-sha', user: 'new-user', password: 'new-password'});
  assert.match(ui.element('credentialsStatus').textContent, /next start/);
});

test('replacing references and discarding unsaved credentials both require confirmation', async () => {
  const ui = setup({mode: 'local', confirmResult: false, credentials: {...credentialFixture, password: {source: 'reference', value: 'env: DEMO_PASSWORD'}}});
  ui.run('state.selectedModule = "demo"');
  await ui.run('openCredentials("build-1")');
  ui.element('credentialsNewPassword').value = 'replacement';
  const count = ui.requests.length;
  await ui.fire('credentialsForm', 'submit');
  assert.equal(ui.requests.length, count);
  ui.fire('cancelCredentials', 'click');
  assert.equal(ui.element('credentialsDialog').open, true);
  assert.equal(ui.element('credentialsNewPassword').value, 'replacement');
});

test('a late credentials response cannot repopulate a closed dialog or a different module', async () => {
  for (const close of [true, false]) {
    let release;
    const ui = setup({mode: 'local', credentials: {...credentialFixture, dashboard_enabled: true, spaces_enabled: false}, credentialsGate: new Promise(resolve => { release = resolve; })});
    ui.run('state.selectedModule = "demo"');
    const opening = ui.run('openCredentials("build-1")');
    if (close) ui.fire('cancelCredentials', 'click');
    else ui.run('state.selectedModule = "other"; state.selectionToken++');
    const flagsBefore = ['credentialsDashboardEnabled', 'credentialsSpacesEnabled'].map(id => ui.element(id).checked);
    release();
    await opening;
    assert.equal(ui.element('credentialsPassword').value, '');
    assert.equal(ui.element('saveCredentials').disabled, true);
    assert.deepEqual(['credentialsDashboardEnabled', 'credentialsSpacesEnabled'].map(id => ui.element(id).checked), flagsBefore);
    assert.equal(ui.element('credentialsDashboardEnabled').disabled, true);
    assert.equal(ui.element('credentialsSpacesEnabled').disabled, true);
  }
});

test('credential conflict preserves drafts and blocks closing during a save', async () => {
  const ui = setup({mode: 'local', credentials: credentialFixture});
  ui.run('state.selectedModule = "demo"');
  await ui.run('openCredentials("build-1")');
  ui.element('credentialsNewPassword').value = 'unsaved';
  ui.element('credentialsSpacesEnabled').checked = false;
  let reject;
  ui.expose('saving', new Promise((resolve, fail) => { reject = fail; }));
  ui.run('apiRequest = () => saving');
  const saving = ui.fire('credentialsForm', 'submit');
  ui.fire('credentialsDialog', 'cancel');
  assert.equal(ui.element('credentialsDialog').open, true);
  assert.equal(ui.element('cancelCredentials').disabled, true);
  reject(new Error('package configuration changed; reload before saving'));
  await saving;
  assert.match(ui.element('credentialsError').textContent, /reload/);
  assert.equal(ui.element('credentialsNewPassword').value, 'unsaved');
  assert.equal(ui.element('credentialsPassword').value, 'demo-password');
  assert.equal(ui.element('credentialsSpacesEnabled').checked, false);
  assert.equal(ui.element('saveCredentials').disabled, false);
});

test('revoking deployment authentication clears credentials even with unsaved changes', async () => {
  const ui = setup({saved: 'test-secret', confirmResult: false, credentials: credentialFixture});
  await flush();
  ui.run('state.selectedModule = "demo"');
  await ui.run('openCredentials("build-1")');
  ui.element('credentialsNewPassword').value = 'draft';
  ui.run('setConnectionState("invalid")');
  assert.equal(ui.element('credentialsDialog').open, false);
  assert.equal(ui.element('credentialsPassword').value, '');
  assert.equal(ui.element('credentialsNewPassword').value, '');
});

test('credentials action is present for stopped and Live builds independently of dashboard availability', () => {
  for (const mode of ['local', 'remote']) {
    const ui = setup({mode});
    ui.run(`
      testRows = [];
      document.createElement = () => ({innerHTML: "", classList: {add() {}}, querySelectorAll: () => []});
      ui.buildRows.appendChild = row => testRows.push(row);
      wireRowMenus = () => {};
      state.builds = [{build_id: "build-1", runtime_mode: "live"}];
      renderBuilds();
    `);
    assert.match(ui.run('testRows[0].innerHTML'), /Developer access/);
    assert.doesNotMatch(ui.run('testRows[0].innerHTML'), /Open dashboard/);
    assert.equal(ui.run('credentialsPath("module name", "build/id")'), (mode === 'local' ? '/local' : '/deploy') + '/modules/module%20name/builds/build%2Fid/credentials');
  }
});

test('closing credentials opened through a row action restores focus to its visible menu toggle', async () => {
  const ui = setup({mode: 'local', credentials: credentialFixture});
  let menuHidden = false, focusTarget = '', clickCredentials;
  const toggle = {setAttribute() {}, focus() { focusTarget = 'toggle'; }};
  const action = {
    disabled: false,
    dataset: {action: 'credentials'},
    addEventListener(name, callback) { if (name === 'click') clickCredentials = callback; },
    focus() { if (!menuHidden) focusTarget = 'action'; }
  };
  const panel = {
    style: {},
    classList: {
      add(name) { if (name === 'hidden') menuHidden = true; },
      remove() {}
    }
  };
  const row = {
    innerHTML: '',
    classList: {add() {}},
    querySelector: selector => selector === '.row-menu-toggle' ? toggle : null,
    querySelectorAll: selector => selector === 'button[data-action]' ? [action] : []
  };
  ui.expose('credentialTestRow', row);
  ui.expose('credentialTestPanel', panel);
  ui.expose('credentialTestToggle', toggle);
  ui.run(`
    document.createElement = () => credentialTestRow;
    document.querySelectorAll = selector => selector === ".row-menu-panel" ? [credentialTestPanel]
      : selector === ".row-menu-toggle" ? [credentialTestToggle] : [];
    ui.buildRows.appendChild = () => {};
    wireRowMenus = () => {};
    state.selectedModule = "demo";
    state.builds = [{build_id: "build-1", runtime_mode: "live"}];
    renderBuilds();
  `);
  clickCredentials();
  await flush();
  assert.equal(menuHidden, true);
  assert.equal(ui.element('credentialsDialog').open, true);
  assert.equal(ui.run('state.credentialsTrigger === credentialTestToggle'), true);
  ui.fire('cancelCredentials', 'click');
  assert.equal(ui.element('credentialsDialog').open, false);
  assert.equal(focusTarget, 'toggle');
});

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

test('header navigation owns Modules and Shared Plugins in both deployment modes', async () => {
  assert.match(html, /<header[^>]*>[\s\S]*id="deploymentNav"[^>]*role="tablist"/);
  assert.equal((html.match(/id="viewModules"/g) || []).length, 1);
  assert.equal((html.match(/id="viewPlugins"/g) || []).length, 1);
  assert.match(html, /id="viewPlugins"[^>]*>[\s\S]*?Shared Plugins<\/button>/);
  assert.doesNotMatch(html, /hb-deploy-tabs/);
  for (const mode of ['local', 'remote']) {
    const ui = setup({mode});
    if (mode === 'remote') {
      assert.equal(ui.element('deploymentNav').hidden, true);
      ui.run('setConnectionState("ready")');
    }
    assert.equal(ui.element('deploymentNav').hidden, false);
    ui.fire('viewPlugins', 'click');
    assert.equal(ui.element('viewPlugins').attrs.get('aria-selected'), 'true');
    assert.equal(ui.element('viewModules').tabIndex, -1);
    assert.equal(ui.element('pluginsView').classList.contains('hidden'), false);
    ui.fire('viewPlugins', 'keydown', {key: 'Home'});
    assert.equal(ui.element('viewModules').attrs.get('aria-selected'), 'true');
    assert.equal(ui.element('viewModules').tabIndex, 0);
    assert.equal(ui.focused(), 'viewModules');
  }
});

test('both deployment drawers use the 1050px breakpoint and the redundant heading is gone', () => {
  assert.match(html, /matchMedia\("\(min-width: 1050px\)"\)/);
  assert.match(css, /@media \(max-width:1049px\)/);
  assert.doesNotMatch(html, /id="deployPageHeading"|id="deploySubtitle"|id="deployMode"/);
  assert.match(html, /id="pluginsView" class="drawer hb-deploy-plugins hidden"/);
});

test('one compact API indicator serves both workspaces and details remain in Connection settings', () => {
  const header = html.slice(html.indexOf('<header'), html.indexOf('</header>'));
  assert.match(header, /id="apiHealth"[^>]*role="status"[^>]*aria-live="polite"/);
  assert.match(header, /id="apiStatusDot"[^>]*aria-hidden="true"/);
  assert.match(header, /class="hb-deploy-health-label">DEPLOY API ·<\/span>/);
  for (const id of ['apiHostMode', 'apiTime', 'apiVersion']) {
    assert.equal((html.match(new RegExp(`id="${id}"`, 'g')) || []).length, 1);
  }
  assert.match(header, /id="menuPanel"[\s\S]*id="apiTime"[\s\S]*id="apiVersion"/);
  assert.doesNotMatch(html, /hb-deploy-status|id="apiStatus"|id="apiStatusPlugins"|id="apiTimePlugins"|id="apiVersionPlugins"/);
  assert.doesNotMatch(html, /<h2>Global Plugins<\/h2>|<h2>Custom Plugins<\/h2>|<h2>Builds<\/h2>/);
});

test('the shared API pill shows deployment mode while its dot and accessible text track health', async () => {
  for (const mode of ['local', 'remote']) {
    const ui = setup({mode, saved: mode === 'remote' ? 'test-secret' : ''});
    const hostMode = mode === 'local' ? 'Local' : 'Remote';
    assert.equal(ui.element('apiHostMode').textContent, hostMode, 'mode is correct before the first API reply');
    await flush();
    const label = html.match(/class="hb-deploy-health-label">([^<]+)<\/span>/)[1];
    assert.equal(ui.element('apiVersion').textContent, 'test');
    assert.notEqual(ui.element('apiTime').textContent, '-');
    for (const health of ['online', 'error 503', 'offline']) {
      if (health !== 'online') ui.run(`setApiStatus(${JSON.stringify(health)})`);
      assert.equal(ui.element('apiHostMode').textContent, hostMode);
      assert.equal(`${label} ${ui.element('apiHostMode').textContent}`, `DEPLOY API · ${hostMode}`);
      assert.equal(ui.element('apiHealth').attrs.get('aria-label'), `Deploy API ${hostMode}: ${health}`);
      assert.equal(ui.element('apiHealth').title, `${hostMode} deployment API: ${health}`);
      assert.equal(ui.element('apiStatusDot').classList.contains('status-success'), health === 'online');
      assert.equal(ui.element('apiStatusDot').classList.contains('status-error'), health !== 'online');
    }
  }
});

test('module actions live in its navbar menu and Shared Plugins keeps its own Refresh action', () => {
  assert.equal((html.match(/id="refreshPlugins"/g) || []).length, 1);
  const modules = html.slice(html.indexOf('id="modulesView"'), html.indexOf('id="pluginsView"'));
  const shared = html.slice(html.indexOf('id="pluginsView"'), html.indexOf('</main>'));
  assert.match(modules, /id="moduleActionsToggle"/);
  assert.match(modules, /id="moduleActionsMenu"[\s\S]*id="openUpload"[\s\S]*id="rollbackModule"/);
  assert.match(modules, /id="moduleTabOverview"[\s\S]*id="moduleTabPlugins"/);
  assert.match(modules, /id="moduleOverviewPanel"[\s\S]*id="moduleCustomPluginsPanel"/);
  assert.doesNotMatch(shared, /id="customPluginSection"|id="customPluginActions"|id="pluginBuild"/);
  for (const mode of ['local', 'remote']) {
    const ui = setup({mode});
    assert.equal(ui.element('rollbackModule').disabled, true);
    ui.run('state.selectedModule = "demo"; state.currentBuild = "current"; state.builds = [{build_id:"previous"},{build_id:"current"}]; updateRollbackTarget()');
    assert.equal(ui.element('rollbackModule').disabled, mode === 'local');
    assert.equal(ui.element('rollbackTarget').title, mode === 'local' ? undefined : 'previous');
  }
});

test('module Actions trigger exposes one responsive label at a time', () => {
  const modules = html.slice(html.indexOf('id="modulesView"'), html.indexOf('id="pluginsView"'));
  const trigger = modules.match(/<summary[^>]*id="moduleActionsToggle"[^>]*>([\s\S]*?)<\/summary>/)?.[1];
  assert.ok(trigger, 'module actions use a native disclosure trigger');
  assert.match(trigger, /id="moduleActionsIcon"/);
  assert.match(trigger, /id="moduleActionsLabel"/);
  assert.match(modules, /id="moduleActionsToggle"[^>]*aria-label="Module actions"/);
  const mobileStart = css.indexOf('@media (max-width:1049px)');
  assert.ok(mobileStart > 0, 'the action trigger shares the drawer breakpoint');
  const desktop = css.slice(0, mobileStart);
  const mobile = css.slice(mobileStart);
  assert.ok(/#moduleActionsIcon\s*\{[^}]*display:\s*none/.test(desktop), 'wide Actions shows text, not its icon');
  assert.ok(/#moduleActionsIcon\s*\{[^}]*display:\s*(?:inline-flex|inline|flex|block)/.test(mobile), 'narrow Actions shows its icon');
  assert.ok(/#moduleActionsLabel\s*\{[^}]*display:\s*none/.test(mobile), 'narrow Actions hides its text');
});

test('Deploy branding and process-stop copy describe the actual host-wide action', () => {
  assert.match(html, /<a class="hb-brand"[^>]*aria-label="HyperBricks Deploy home"[^>]*>[\s\S]*?<span>Deploy<\/span><\/a>/);
  assert.match(html, /id="killAllProcesses"[^>]*>Stop other HyperBricks processes<\/button>/);
  assert.match(source, /Stop other HyperBricks processes on this host\? This may stop module runtimes and unrelated HyperBricks commands\. The Deploy API stays running\./);
  assert.doesNotMatch(html, /Stop running builds|Kill all hyperbricks/);
});

test('developer access names Dashboard as the Overview and Errors switch, with Spaces separate', () => {
  assert.match(html, /<label for="credentialsDashboardEnabled">Dashboard<\/label>/);
  assert.match(html, /id="dashboardVisibilityHelp"[^>]*>Show the Overview and Errors views in Development mode\. Spaces has its own switch\./);
  assert.match(html, /<label for="credentialsSpacesEnabled">Spaces<\/label>/);
  assert.match(html, /id="spacesVisibilityHelp"[^>]*>Show the Spaces editor in Development mode without changing other frontend editors\./);
});

test('small explanations use hover, touch and keyboard help popovers while important state remains visible', () => {
  for (const id of [
    'requestSigningHelp', 'signerHelp', 'localSyncHelp', 'stopProcessesHelp',
    'developerAccessHelp', 'credentialsScope', 'dashboardVisibilityHelp',
    'spacesVisibilityHelp', 'credentialStorageHelp'
  ]) {
    const trigger = html.match(new RegExp(`<button[^>]*data-help-popover="${id}"[^>]*>`))?.[0];
    assert.ok(trigger, `${id} needs a help trigger`);
    assert.match(trigger, /aria-label="[^"]+"/);
    assert.match(trigger, new RegExp(`aria-controls="${id}"`));
    assert.match(trigger, /aria-expanded="false"/);
    assert.match(html, new RegExp(`id="${id}" popover="manual" role="tooltip"`));
  }
  assert.match(source, /function wireHelpPopovers\(\)/);
  assert.match(source, /trigger\.addEventListener\("pointermove"/);
  assert.match(source, /trigger\.addEventListener\("click"/);
  assert.doesNotMatch(source, /trigger\.addEventListener\("focus",\s*\(\)\s*=>\s*showHelpPopover/);
  assert.match(html, /id="credentialsSpacesParentWarning" hidden/);
  assert.match(html, /Stored as plain text · restart to apply\./);
  assert.match(css, /\.hb-deploy-shell\s*\{[^}]*min-width:\s*300px/);
});

test('Kill and Delete build use danger text while Stop stays neutral', () => {
  const kill = html.match(/<button[^>]*id="killAllProcesses"[^>]*>/)?.[0];
  assert.ok(kill);
  assert.match(kill, /class="[^"]*\bhb-danger-action\b/);
  assert.doesNotMatch(kill, /\bbtn-error\b/);
  assert.match(html, /action: "stop",\s*label: "Stop",\s*icon: icons\.stop,\s*attrs: buildAttr/);
  assert.match(html, /action: "delete",\s*label: "Delete build",\s*icon: icons\.delete,\s*className: "hb-danger-action"/);
  assert.match(css, /\.hb-danger-action:not\(:disabled\)\s*\{[^}]*color:\s*color-mix\(in oklab,var\(--color-error\) 70%,var\(--color-base-content\)\)/);
  assert.match(html, /action: "remove",[\s\S]*?className: "btn-error"/, 'plugin Remove retains its existing treatment');
  for (const mode of ['local', 'remote']) {
    const ui = setup({mode});
    ui.run(`
      testRows = [];
      document.createElement = () => ({innerHTML: "", classList: {add() {}}, querySelectorAll: () => []});
      ui.buildRows.appendChild = row => testRows.push(row);
      wireRowMenus = () => {};
      state.builds = [{build_id: "build-1", runtime_mode: "development", format: "hra"}];
      updateStatus({running: true, running_build: "build-1", running_mode: "development"});
      renderBuilds();
    `);
    const row = ui.run('testRows[0].innerHTML');
    const stopClass = row.match(/<button class="([^"]*)"[^>]*data-action="stop"/)?.[1];
    const deleteClass = row.match(/<button class="([^"]*)"[^>]*data-action="delete"/)?.[1];
    assert.ok(stopClass);
    assert.ok(deleteClass);
    assert.doesNotMatch(stopClass, /hb-danger-action|btn-error/, `${mode} Stop stays neutral`);
    assert.match(deleteClass, /hb-danger-action/, `${mode} Delete uses danger text`);
  }
});

test('both workspaces have native, collapsible sticky Activity panels with a latest-message preview', () => {
  for (const [panelID, previewID, outputID] of [
    ['moduleActivityPanel', 'moduleActivityLatest', 'activity'],
    ['pluginActivityPanel', 'pluginActivityLatest', 'pluginActivity']
  ]) {
    const panel = html.match(new RegExp(`<details[^>]*id="${panelID}"[^>]*>([\\s\\S]*?)<\\/details>`));
    assert.ok(panel, `${panelID} must use native details for Enter/Space disclosure`);
    assert.doesNotMatch(panel[0].slice(0, panel[0].indexOf('>')), /\sopen(?:\s|=|>)/);
    const summary = panel[1].match(/<summary[^>]*>([\s\S]*?)<\/summary>/)?.[1];
    assert.ok(summary, `${panelID} needs a native summary`);
    assert.match(summary, /Activity/);
    assert.match(summary, new RegExp(`id="${previewID}"`));
    assert.match(panel[1], new RegExp(`<pre[^>]*id="${outputID}"`));
  }
  assert.ok(/\.hb-deploy-log\s*\{[^}]*position:\s*sticky;[^}]*bottom:\s*0/.test(css), 'Activity sticks to the workspace bottom');
});

test('mobile deployment height follows the wrapped header instead of a fixed header offset', () => {
  assert.match(html, /<body[^>]*class="hb-deploy-shell"/);
  assert.doesNotMatch(css, /100dvh\s*-\s*130px/, 'the mobile header height is content-dependent');
  assert.match(css, /\.hb-deploy-shell\s*\{[^}]*display:\s*flex;[^}]*flex-direction:\s*column/);
  assert.match(css, /#deploymentContent:not\(\[hidden\]\)\s*\{[^}]*flex:\s*1/);
  assert.match(css, /\.hb-deploy-workspace\s*\{[^}]*flex:\s*1;[^}]*min-height:\s*auto/);
});

test('Activity preview mirrors success and error messages without forcing disclosure open', () => {
  for (const mode of ['local', 'remote']) {
    const ui = setup({mode});
    ui.run('setActivity("Build ready.")');
    assert.equal(ui.element('activity').textContent, 'Build ready.');
    assert.equal(ui.element('pluginActivity').textContent, 'Build ready.');
    assert.equal(ui.element('moduleActivityLatest').textContent, 'Build ready.');
    assert.equal(ui.element('pluginActivityLatest').textContent, 'Build ready.');
    assert.equal(ui.element('moduleActivityPanel').open, false);
    assert.equal(ui.element('pluginActivityPanel').open, false);

    ui.element('moduleActivityPanel').open = true;
    ui.run('setActivity("Could not restart.", true)');
    assert.equal(ui.element('activity').textContent, 'Could not restart.');
    assert.equal(ui.element('pluginActivity').textContent, 'Could not restart.');
    assert.equal(ui.element('moduleActivityLatest').textContent, 'Could not restart.');
    assert.equal(ui.element('pluginActivityLatest').textContent, 'Could not restart.');
    assert.equal(ui.element('deployFeedbackText').textContent, 'Could not restart.');
    assert.equal(ui.element('moduleActivityPanel').classList.contains('is-error'), true);
    assert.equal(ui.element('pluginActivityPanel').classList.contains('is-error'), true);
    assert.equal(ui.element('moduleActivityPanel').open, true);
    assert.equal(ui.element('pluginActivityPanel').open, false);
  }
});

test('deployment tables wrap into subtly contrasted cards below 710px', () => {
  const cardStart = css.indexOf('@media (max-width:710px)');
  const mobileStart = css.indexOf('@media (max-width:540px)');
  assert.ok(cardStart > 0 && mobileStart > cardStart);
  const wideStyles = css.slice(0, cardStart);
  const cardStyles = css.slice(cardStart, mobileStart);
  assert.match(wideStyles, /\.hb-deploy-page \.table \.badge\s*\{[^}]*white-space:\s*nowrap/);
  const pluginWidth = wideStyles.match(/\.hb-deploy-global \.table,\s*\.hb-deploy-custom \.table\s*\{[^}]*min-width:\s*(\d+)px/);
  assert.ok(pluginWidth && Number(pluginWidth[1]) >= 600);
  assert.match(cardStyles, /\.hb-deploy-page \.table thead\s*\{[^}]*display:\s*none/);
  assert.match(cardStyles, /\.hb-deploy-builds \.table,\s*\.hb-deploy-global \.table,\s*\.hb-deploy-custom \.table\s*\{[^}]*min-width:\s*0/);
  assert.match(cardStyles, /\.hb-deploy-page \.table tr\s*\{[^}]*display:\s*grid/);
  assert.match(cardStyles, /\.hb-deploy-builds \.table tbody tr:not\(\.details-row\),\s*\.hb-deploy-global \.table tbody tr,\s*\.hb-deploy-custom \.table tbody tr\s*\{[^}]*border:\s*1px solid var\(--color-base-300\)[^}]*background:\s*var\(--color-base-100\)/);
  assert.match(wideStyles, /\.hb-module-picker\s*\{[^}]*position:\s*sticky;\s*top:\s*0/);
  assert.match(wideStyles, /\.hb-module-picker>nav\s*\{[^}]*overflow-y:\s*auto/);
  assert.doesNotMatch(css.slice(mobileStart), /\.hb-deploy-health strong\s*\{[^}]*display:\s*none/);
});

test('compact plugin rows stay grouped as cards without internal cell borders', () => {
  const cardStyles = css.slice(css.indexOf('@media (max-width:710px)'), css.indexOf('@media (max-width:540px)'));
  const cardRule = cardStyles.match(/\.hb-deploy-builds \.table tbody tr:not\(\.details-row\),\s*\.hb-deploy-global \.table tbody tr,\s*\.hb-deploy-custom \.table tbody tr\s*\{([^}]+)\}/);
  assert.ok(cardRule, 'all deployment tables must group each row at the compact breakpoint');
  assert.match(cardRule[1], /border:\s*1px solid/);
  assert.match(cardRule[1], /border-radius:\s*[^;]+/);
  assert.match(cardRule[1], /(?:margin(?:-block|-bottom)?:|gap:)\s*(?!0(?:px)?[;\s])[^;]+/);
  const cellRule = cardStyles.match(/\.hb-deploy-global \.table (?:tbody )?td,\s*\.hb-deploy-custom \.table (?:tbody )?td\s*\{([^}]+)\}/);
  assert.ok(cellRule, 'both plugin card types must suppress the table cell borders');
  assert.match(cellRule[1], /border:\s*(?:0|none)/);
});

test('mobile module drawer opens with search focus and closes by Escape, view switch or breakpoint', () => {
  const ui = setup({mode: 'local', narrow: true});
  assert.equal(ui.element('modulesView').classList.contains('drawer-open'), false);
  ui.fire('moduleDrawerOpen', 'click');
  assert.equal(ui.element('moduleDrawerToggle').checked, true);
  assert.equal(ui.element('moduleDrawerOpen').attrs.get('aria-expanded'), 'true');
  assert.equal(ui.element('moduleSidebar').attrs.get('role'), 'dialog');
  assert.equal(ui.element('moduleDrawerContent').inert, true);
  assert.equal(ui.focused(), 'moduleSearch');
  ui.fire('moduleSidebar', 'keydown', {key: 'Escape'});
  assert.equal(ui.element('moduleDrawerToggle').checked, false);
  assert.equal(ui.element('moduleDrawerContent').inert, false);
  assert.equal(ui.focused(), 'moduleDrawerOpen');
  ui.fire('moduleDrawerOpen', 'click');
  ui.fire('viewPlugins', 'click');
  assert.equal(ui.element('moduleDrawerToggle').checked, false);
  ui.fire('viewModules', 'click');
  ui.fire('moduleDrawerOpen', 'click');
  ui.setNarrow(false);
  assert.equal(ui.element('modulesView').classList.contains('drawer-open'), true);
  assert.equal(ui.element('moduleDrawerToggle').checked, false);
  ui.setNarrow(true);
  assert.equal(ui.element('modulesView').classList.contains('drawer-open'), false);
});

test('mobile plugin drawer opens with search focus and closes by Escape or breakpoint', () => {
  const ui = setup({mode: 'local', narrow: true});
  ui.fire('viewPlugins', 'click');
  ui.fire('pluginDrawerOpen', 'click');
  assert.equal(ui.element('pluginDrawerToggle').checked, true);
  assert.equal(ui.element('pluginDrawerOpen').attrs.get('aria-expanded'), 'true');
  assert.equal(ui.element('pluginSidebar').attrs.get('role'), 'dialog');
  assert.equal(ui.element('pluginDrawerContent').inert, true);
  assert.equal(ui.focused(), 'pluginSearch');
  ui.fire('pluginSidebar', 'keydown', {key: 'Escape'});
  assert.equal(ui.element('pluginDrawerToggle').checked, false);
  assert.equal(ui.element('pluginDrawerContent').inert, false);
  assert.equal(ui.focused(), 'pluginDrawerOpen');
  ui.fire('pluginDrawerOpen', 'click');
  ui.setNarrow(false);
  assert.equal(ui.element('pluginsView').classList.contains('drawer-open'), true);
  assert.equal(ui.element('pluginDrawerToggle').checked, false);
});

test('Shared Plugins drawer contains only shared entries even when module inventory is cached', () => {
  const ui = setup({mode: 'local'});
  ui.run(`
    state.modules = ['module-one', 'module-two'];
    state.globalPluginEntries = [
      {key: 'global:alpha', name: 'Alpha', version: '1.0', status: 'installed'},
      {key: 'global:beta', name: 'Beta', version: '2.0', status: 'missing'}
    ];
    renderCustomPlugins([{name: 'Gamma__module-two', version: '3.0'}], 'module-two');
    const alpha = document.createElement('tr');
    alpha.dataset.pluginKey = 'global:alpha';
    ui.globalPluginRows.appendChild(alpha);
    const beta = document.createElement('tr');
    beta.dataset.pluginKey = 'global:beta';
    ui.globalPluginRows.appendChild(beta);
    renderPluginSidebar();
  `);
  assert.equal(ui.element('pluginList').children.length, 2);
  assert.equal(ui.element('pluginList').children[0].children[0].children[0].textContent, 'Alpha');
  ui.element('pluginList').children[1].children[0].listeners.get('click')();
  assert.equal(ui.run('state.pluginKey'), 'global:beta');
  assert.equal(ui.element('selectedPluginTitle').textContent, 'Beta · 2.0');
  assert.equal(ui.element('globalPluginRows').children[0].classList.contains('hidden'), true);
  assert.equal(ui.element('globalPluginRows').children[1].classList.contains('hidden'), false);
  ui.element('pluginSearch').value = 'module-two';
  ui.fire('pluginSearch', 'input');
  assert.equal(ui.element('pluginList').children.some(node => node.children?.[0]?.children?.[0]?.textContent === 'Gamma'), false);
  assert.match(ui.element('pluginList').children.at(-1).textContent, /No shared plugins match/);
});

test('module custom plugin view strips the display suffix but preserves its full config name', () => {
  const ui = setup({mode: 'local'});
  ui.run(`
    state.modules = ['hyperbricks-patterns-yaml'];
    state.selectedModule = 'hyperbricks-patterns-yaml';
    state.pluginModule = 'hyperbricks-patterns-yaml';
    const plugin = {
      name: 'WorkflowActionsDemoPlugin__hyperbricks-patterns-yaml',
      module: 'hyperbricks-patterns-yaml',
      version: '1.0.0',
      config_name: 'WorkflowActionsDemoPlugin__hyperbricks-patterns-yaml@1.0.0',
      status: 'installed'
    };
    renderCustomPlugins([plugin], 'hyperbricks-patterns-yaml');
  `);
  assert.equal(ui.run('customPluginDisplayName("WorkflowActionsDemoPlugin__hyperbricks-patterns-yaml", "hyperbricks-patterns-yaml")'), 'WorkflowActionsDemoPlugin');
  assert.match(ui.element('customPluginRows').children[0].innerHTML, /WorkflowActionsDemoPlugin__hyperbricks-patterns-yaml@1\.0\.0/);
  assert.equal(ui.run('state.customPluginEntries[0].configName'), 'WorkflowActionsDemoPlugin__hyperbricks-patterns-yaml@1.0.0');
  assert.equal(ui.element('pluginList').children.some(node => node.textContent.includes('WorkflowActionsDemoPlugin')), false);
});

test('module Custom plugins tab loads only its selected local module without a build ID', async () => {
  const ui = setup({mode: 'local', responseForRequest: url => {
    if (!url.pathname.endsWith('/plugins/custom')) return undefined;
    const module = url.searchParams.get('module');
    return {plugins: [{name: `Plugin__${module}`, module, version: '1.0.0', status: 'installed'}]};
  }});
  ui.run(`state.modules = ['alpha', 'beta', 'gamma']`);
  await ui.run('selectModule("alpha")');
  assert.equal(ui.requests.some(request => new URL(request.url).pathname.endsWith('/plugins/custom')), false,
    'Overview must not fetch custom plugins eagerly');
  ui.fire('moduleTabPlugins', 'click');
  await flush();
  await flush();
  const calls = ui.requests.filter(request => new URL(request.url).pathname.endsWith('/plugins/custom'));
  assert.ok(calls.length > 0);
  assert.deepEqual([...new Set(calls.map(request => new URL(request.url).searchParams.get('module')))], ['alpha']);
  assert.equal(calls.every(request => !new URL(request.url).searchParams.has('build_id')), true);
  assert.match(ui.element('customPluginRows').children[0].innerHTML, /data-label="Plugin">Plugin</);
  assert.match(ui.element('customPluginRows').children[0].innerHTML, /data-module="alpha"/);
  assert.equal(ui.element('pluginList').children.some(node => node.textContent.includes('alpha')), false);
});

test('saved local Shared Plugins view loads module names but no module plugin inventory', async () => {
  const ui = setup({mode: 'local', savedView: 'plugins', stubModuleLoad: false, stubGlobalLoad: false,
    responseForRequest: url => {
      if (url.pathname === '/local/plugins/global/index') return {plugins: {}};
      if (url.pathname === '/local/plugins/global') return {plugins: [
        {name: 'Shared', config_name: 'Shared@1.0', version: '1.0'}
      ]};
      return undefined;
    }});
  await ui.run('state.pluginLoadPromise');
  assert.equal(ui.requests.filter(request => new URL(request.url).pathname === '/local/modules').length, 1);
  assert.equal(ui.requests.some(request => new URL(request.url).pathname === '/local/plugins/custom'), false);
  assert.equal(ui.run('state.globalPluginEntries.length'), 1);
  assert.equal(ui.element('pluginSummary').textContent, '1 shared plugin');
});

test('manual Modules reload while Shared Plugins is open does not fetch module plugins', async () => {
  let moduleLists = 0;
  const ui = setup({mode: 'local', savedView: 'plugins', stubModuleLoad: false, stubGlobalLoad: false,
    responseForRequest: url => {
      if (url.pathname === '/local/modules') {
        moduleLists++;
        return {modules: moduleLists === 1 ? ['alpha', 'beta'] : ['alpha', 'gamma']};
      }
      if (url.pathname === '/local/plugins/global/index') return {plugins: {}};
      if (url.pathname === '/local/plugins/global') return {plugins: []};
      return undefined;
    }});
  await ui.run('state.pluginLoadPromise');
  await ui.fire('loadModules', 'click');
  assert.equal(moduleLists, 2);
  assert.equal(ui.requests.some(request => new URL(request.url).pathname === '/local/plugins/custom'), false);
  assert.equal(ui.element('pluginList').children.some(node => /alpha|beta|gamma/.test(node.textContent)), false);
  assert.equal(ui.element('pluginSummary').textContent, '0 shared plugins');
});

test('a missing shared catalog keeps installed entries and exposes a retry warning', async () => {
  let catalogCalls = 0;
  const ui = setup({mode: 'local', savedView: 'plugins', stubModuleLoad: false, stubGlobalLoad: false,
    responseForRequest: url => {
      if (url.pathname === '/local/modules') return {modules: ['alpha', 'beta']};
      if (url.pathname === '/local/plugins/global/index') {
        catalogCalls++;
        return catalogCalls === 1 ? {status: 503, error: 'catalog unavailable'} : {plugins: {}};
      }
      if (url.pathname === '/local/plugins/global') {
        return {plugins: [{name: 'Installed', config_name: 'Installed@1.0', version: '1.0'}]};
      }
      if (url.pathname === '/local/plugins/custom') return {plugins: []};
      return undefined;
    }});
  await ui.run('state.pluginLoadPromise');
  assert.equal(ui.run('state.globalPluginEntries[0].name'), 'Installed');
  assert.equal(ui.element('pluginList').children[0].children[0].children[0].textContent, 'Installed');
  assert.match(ui.element('activity').textContent, /Shared plugin catalog unavailable.*Refresh to retry/);
  await ui.run('autoLoadPlugins(true)');
  assert.equal(catalogCalls, 2);
  assert.equal(ui.element('activity').textContent, 'Shared plugins updated.');
});

test('switching modules while Custom plugins is open replaces the module rows', async () => {
  const ui = setup({mode: 'local', responseForRequest: url => {
    if (url.pathname.endsWith('/plugins/custom')) {
      const module = url.searchParams.get('module');
      return {plugins: [{name: `Plugin__${module}`, module, version: '1.0'}]};
    }
    return undefined;
  }});
  ui.run(`state.modules = ['alpha', 'beta']`);
  await ui.run('selectModule("alpha")');
  ui.fire('moduleTabPlugins', 'click');
  await flush();
  await flush();
  assert.match(ui.element('customPluginRows').children[0].innerHTML, /data-module="alpha"/);
  await ui.run('selectModule("beta")');
  await flush();
  await flush();
  assert.match(ui.element('customPluginRows').children[0].innerHTML, /data-module="beta"/);
  assert.doesNotMatch(ui.element('customPluginRows').children[0].innerHTML, /data-module="alpha"/);
  assert.equal(ui.element('moduleTabPlugins').attrs.get('aria-selected'), 'true');
  assert.equal(ui.requests.filter(request => new URL(request.url).pathname.endsWith('/plugins/custom'))
    .every(request => ['alpha', 'beta'].includes(new URL(request.url).searchParams.get('module'))), true);
});

test('module Overview and Custom plugins tabs expose separate panels and keyboard selection', () => {
  for (const mode of ['local', 'remote']) {
    const ui = setup({mode});
    ui.run('setModuleTab("overview")');
    assert.equal(ui.element('moduleTabOverview').attrs.get('aria-selected'), 'true');
    assert.equal(ui.element('moduleOverviewPanel').classList.contains('hidden'), false);
    ui.fire('moduleTabPlugins', 'click');
    assert.equal(ui.element('moduleTabPlugins').attrs.get('aria-selected'), 'true');
    assert.equal(ui.element('moduleTabOverview').tabIndex, -1);
    assert.equal(ui.element('moduleOverviewPanel').classList.contains('hidden'), true);
    assert.equal(ui.element('moduleCustomPluginsPanel').classList.contains('hidden'), false);
    ui.fire('moduleTabPlugins', 'keydown', {key: 'Home'});
    assert.equal(ui.element('moduleTabOverview').attrs.get('aria-selected'), 'true');
    assert.equal(ui.focused(), 'moduleTabOverview');
  }
});

test('remote Custom plugins tab uses the selected module and selected build only', async () => {
  const ui = setup({mode: 'remote', saved: 'test-secret', stubPluginBuilds: false,
    responseForRequest: url => url.pathname.endsWith('/plugins/custom') ? {plugins: []} : undefined});
  await flush();
  ui.run(`
    state.modules = ['alpha', 'beta'];
    state.selectedModule = 'alpha';
    state.pluginModule = 'alpha';
    state.currentBuild = 'alpha-current';
    state.builds = [
      {build_id: 'alpha-current', built_at: '2026-09-20T10:00:00Z'},
      {build_id: 'alpha-newer', built_at: '2026-09-24T10:00:00Z'}
    ];
    renderPluginBuilds();
  `);
  ui.fire('moduleTabPlugins', 'click');
  await flush();
  await flush();
  let calls = ui.requests.filter(request => new URL(request.url).pathname.endsWith('/plugins/custom'));
  assert.ok(calls.length > 0);
  assert.equal(calls.every(request => new URL(request.url).searchParams.get('module') === 'alpha'), true);
  assert.equal(new URL(calls.at(-1).url).searchParams.get('build_id'), 'alpha-current');
  ui.element('pluginBuild').value = 'alpha-newer';
  ui.fire('pluginBuild', 'change');
  await flush();
  await flush();
  calls = ui.requests.filter(request => new URL(request.url).pathname.endsWith('/plugins/custom'));
  assert.equal(new URL(calls.at(-1).url).searchParams.get('build_id'), 'alpha-newer');
  assert.equal(calls.some(request => new URL(request.url).searchParams.get('module') === 'beta'), false);
});

test('Custom plugins tab displays all plugins for its selected module', async () => {
  const module = 'alpha';
  const plugins = [
    {name: 'First__alpha', module, version: '1.0', config_name: 'First__alpha@1.0'},
    {name: 'Second__alpha', module, version: '2.0', config_name: 'Second__alpha@2.0'}
  ];
  const ui = setup({mode: 'local', responseForRequest: url =>
    url.pathname.endsWith('/plugins/custom') ? {plugins} : undefined});
  ui.run(`state.modules = ['alpha']`);
  await ui.run('selectModule("alpha")');
  ui.fire('moduleTabPlugins', 'click');
  await flush();
  await flush();
  assert.equal(ui.element('customPluginRows').children.length, 2);
  assert.equal(ui.element('customPluginRows').children[0].classList.contains('hidden'), false);
  assert.equal(ui.element('customPluginRows').children[1].classList.contains('hidden'), false);
  assert.match(ui.element('customPluginRows').children[0].innerHTML, /First__alpha@1\.0/);
  assert.match(ui.element('customPluginRows').children[1].innerHTML, /Second__alpha@2\.0/);
  assert.equal(ui.element('moduleCustomPluginsPanel').classList.contains('hidden'), false);
  assert.equal(ui.element('pluginList').children.some(node => /First|Second/.test(node.textContent)), false);
});

test('failed selected-module inventory stays inside its Custom plugins panel', async () => {
  const ui = setup({mode: 'local', responseForRequest: url => {
    if (!url.pathname.endsWith('/plugins/custom')) return undefined;
    return url.searchParams.get('module') === 'broken'
      ? {status: 503, error: 'temporarily unavailable'}
      : {plugins: [{name: 'Ready__working', module: 'working', version: '1.0'}]};
  }});
  ui.run(`state.modules = ['broken', 'working']`);
  await ui.run('selectModule("broken")');
  ui.fire('moduleTabPlugins', 'click');
  await flush();
  await flush();
  assert.match(ui.element('activity').textContent, /temporarily unavailable/);
  assert.equal(ui.element('pluginList').children.some(node => /broken|working/.test(node.textContent)), false);
  await ui.run('selectModule("working")');
  await flush();
  await flush();
  assert.match(ui.element('customPluginRows').children[0].innerHTML, /data-label="Plugin">Ready</);
  assert.match(ui.element('customPluginRows').children[0].innerHTML, /data-module="working"/);
});

test('a late previous-module response cannot overwrite the current module plugins', async () => {
  let releaseFirst;
  const firstResponse = new Promise(resolve => { releaseFirst = resolve; });
  let customCalls = 0;
  const ui = setup({mode: 'local', responseForRequest: url => {
    if (!url.pathname.endsWith('/plugins/custom')) return undefined;
    customCalls++;
    return customCalls === 1 ? firstResponse
      : {plugins: [{name: 'Fresh__beta', module: 'beta', version: '2.0'}]};
  }});
  ui.run(`state.modules = ['alpha', 'beta']`);
  await ui.run('selectModule("alpha")');
  ui.fire('moduleTabPlugins', 'click');
  await flush();
  await ui.run('selectModule("beta")');
  await flush();
  releaseFirst({plugins: [{name: 'Stale__alpha', module: 'alpha', version: '1.0'}]});
  await flush();
  assert.equal(customCalls, 2);
  assert.equal(ui.run('state.selectedModule'), 'beta');
  assert.match(ui.element('customPluginRows').children[0].innerHTML, /data-label="Plugin">Fresh</);
  assert.match(ui.element('customPluginRows').children[0].innerHTML, /data-module="beta"/);
  assert.doesNotMatch(ui.element('customPluginRows').children[0].innerHTML, /data-module="alpha"/);
});

test('the sidebar renders the full module list and filters it without changing selection', () => {
  const ui = setup({mode: 'local'});
  ui.run(`
    testModuleRows = [];
    Object.defineProperty(ui.moduleList, "innerHTML", {
      set() { testModuleRows.length = 0; }, get() { return ""; }
    });
    document.createElement = tag => ({
      tag, children: [], attrs: {}, listeners: {},
      appendChild(child) { this.children.push(child); },
      setAttribute(name, value) { this.attrs[name] = value; },
      addEventListener(name, fn) { this.listeners[name] = fn; }
    });
    ui.moduleList.appendChild = item => testModuleRows.push(item);
    state.modules = Array.from({length: 12}, (_, index) => "module-" + index);
    state.selectedModule = "module-11";
    renderModules();
  `);
  assert.equal(ui.run('testModuleRows.length'), 12);
  assert.equal(ui.run('testModuleRows[11].children[0].attrs["aria-current"]'), 'true');
  assert.equal(ui.element('selectedModuleTitle').textContent, 'module-11');
  ui.element('moduleSearch').value = 'module-11';
  ui.fire('moduleSearch', 'input');
  assert.equal(ui.run('testModuleRows.length'), 1);
  assert.equal(ui.element('moduleSummary').textContent, '1 matching module');
  assert.equal(ui.run('state.selectedModule'), 'module-11');
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

test('dashboard action and details use runtime availability, not the saved mode, in both interfaces', () => {
  for (const mode of ['local', 'remote']) {
    const ui = setup({mode});
    ui.run(`
      testRows = [];
      document.createElement = () => ({innerHTML: "", classList: {add() {}}, querySelectorAll: () => []});
      ui.buildRows.appendChild = row => testRows.push(row);
      wireRowMenus = () => {};
      state.builds = [{build_id: "build-1", runtime_mode: "live"}];
      updateStatus({running: true, running_build: "build-1", running_mode: "development", dashboard_path: "/__hyperbricks/dashboard"});
      renderBuilds();
    `);
    // Saved Live may differ from the still-running Development process after a failed restart.
    assert.match(ui.run('testRows[0].innerHTML'), /Open dashboard/);
    const available = {running: true, port: 8123, runtime_mode: 'live', dashboard_path: '/__hyperbricks/dashboard'};
    ui.expose('testBuildStatus', available);
    assert.match(ui.run('buildDetailsHTML("demo", "build-1", testBuildStatus)'), /http:\/\/localhost:8123\/__hyperbricks\/dashboard/);
    for (const status of [
      {running: true, running_build: 'build-1', running_mode: 'live', dashboard_path: ''},
      {running: false, dashboard_path: '/__hyperbricks/dashboard'},
      {running: true, running_build: 'build-1'}
    ]) {
      ui.expose('testStatus', status);
      ui.run('testRows = []; updateStatus(testStatus); renderBuilds();');
      assert.doesNotMatch(ui.run('testRows[0].innerHTML'), /Open dashboard/);
      ui.expose('testBuildStatus', {...status, port: 8123});
      assert.doesNotMatch(ui.run('buildDetailsHTML("demo", "build-1", testBuildStatus)'), /Module Dashboard/);
    }
    ui.run(`
      testRows = [];
      updateStatus({running: true, running_build: "build-1", running_mode: "development", dashboard_path: ""});
      renderBuilds();
    `);
    assert.match(ui.run('testRows[0].innerHTML'), /data-action="dashboard" disabled/);
    assert.match(ui.run('testRows[0].innerHTML'), /Enable development.dashboard.enabled/);
  }
});

test('build details show a shortened origin build ID only for duplicated builds', () => {
  for (const mode of ['local', 'remote']) {
    const ui = setup({mode});
    const origin = '0123456789abcdef';
    ui.expose('testBuildStatus', {build_id: 'new-build', format: 'hra', origin_build_id: origin});
    const duplicated = ui.run('buildDetailsHTML("demo", "new-build", testBuildStatus)');
    assert.match(duplicated, /<span>Origin build<\/span><strong class="mono wrap" title="0123456789abcdef">01234…bcdef<\/strong>/);

    ui.expose('testBuildStatus', {build_id: 'original-build', format: 'hra'});
    assert.doesNotMatch(ui.run('buildDetailsHTML("demo", "original-build", testBuildStatus)'), /Origin build/);
    ui.expose('testBuildStatus', {build_id: 'original-build', format: 'hra', origin_build_id: ''});
    assert.doesNotMatch(ui.run('buildDetailsHTML("demo", "original-build", testBuildStatus)'), /Origin build/);
  }
});

test('opening a dashboard rechecks runtime availability and uses the endpoint returned by the API', async () => {
  for (const mode of ['local', 'remote']) {
    const ui = setup({mode});
    ui.run(`
      state.selectedModule = "demo";
      testOpened = [];
      window.open = (...args) => testOpened.push(args);
      apiRequest = async () => testBuildStatus;
    `);
    ui.expose('testBuildStatus', {running: true, port: 8123, dashboard_path: '/__hyperbricks/dashboard'});
    await ui.run('openBuildLink("build-1", "", "dashboard")');
    assert.equal(ui.run('testOpened[0][0]'), 'http://localhost:8123/__hyperbricks/dashboard');
    for (const status of [
      {running: false, dashboard_path: '/__hyperbricks/dashboard'},
      {running: true, dashboard_path: ''},
      {running: true},
      {running: true, dashboard_path: '//another-host/dashboard'},
      {running: true, dashboard_path: '/\\\\another-host/dashboard'}
    ]) {
      ui.expose('testBuildStatus', {...status, port: 8123});
      await ui.run('openBuildLink("build-1", "", "dashboard")');
      assert.equal(ui.run('testOpened.length'), 1);
      assert.match(ui.element('deployFeedbackText').textContent, /Dashboard unavailable/);
    }
  }
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

test('a saved mode stays selected when its restart fails in either deployment interface', async () => {
  for (const mode of ['local', 'remote']) {
    for (const statusError of ['', 'status unavailable']) {
      const ui = setup({
        mode,
        saved: 'test-secret',
        modeResponse: {runtime_mode: 'live', production: true, restarted: false, restart_error: 'process exited'}
      });
      await flush();
      ui.expose('testStatusError', statusError);
      ui.run(`
        renderBuilds = () => {};
        testStatusRefreshes = 0;
        refreshStatus = async () => {
          testStatusRefreshes++;
          if (testStatusError) throw new Error(testStatusError);
        };
        state.selectedModule = "demo";
        state.selectionToken = 7;
        state.builds = [{build_id: "build-1", runtime_mode: "development", production: false}];
        testModeSelect = {value: "live", disabled: false, isConnected: true};
      `);

      await ui.run('setBuildMode("build-1", "live", testModeSelect)');
      assert.equal(ui.run('state.builds[0].runtime_mode'), 'live');
      assert.equal(ui.run('state.builds[0].production'), true);
      assert.equal(ui.run('testModeSelect.value'), 'live');
      assert.equal(ui.run('testModeSelect.disabled'), false);
      assert.equal(ui.run('testStatusRefreshes'), 1);
      assert.equal(ui.element('deployFeedback').hidden, false);
      assert.match(ui.element('deployFeedbackText').textContent, /^Mode saved as Live, but restart failed: process exited/);
      if (statusError) assert.match(ui.element('deployFeedbackText').textContent, /Status refresh failed: status unavailable/);
    }
  }
});

test('a status refresh error cannot undo a successfully saved mode selection', async () => {
  const ui = setup({mode: 'local', modeResponse: {runtime_mode: 'live', production: true, restarted: true}});
  await flush();
  ui.run(`
    renderBuilds = () => {};
    clearBuildLogOutput = () => {};
    refreshStatus = async () => { throw new Error("status unavailable"); };
    state.selectedModule = "demo";
    state.selectionToken = 7;
    state.builds = [{build_id: "build-1", runtime_mode: "development", production: false}];
    testModeSelect = {value: "live", disabled: false, isConnected: true};
  `);
  await assert.rejects(ui.run('setBuildMode("build-1", "live", testModeSelect)'), /status unavailable/);
  assert.equal(ui.run('state.builds[0].runtime_mode'), 'live');
  assert.equal(ui.run('testModeSelect.value'), 'live');
  assert.equal(ui.run('testModeSelect.disabled'), false);
});

test('package editor shows the saved build mode without changing the raw YAML', async () => {
  for (const mode of ['local', 'remote']) {
    const source = '# preserved\r\nhyperbricks:\r\n  mode: development\r\n';
    const editor = createEditorHarness();
    const packageConfig = {content: source, sha256: 'sha', scope: 'runtime', runtime_mode: 'live', restart_required: false};
    const ui = setup({mode, saved: 'test-secret', editorModule: editor.module, packageConfig});
    await flush();
    ui.run('state.selectedModule = "demo"; state.selectionToken = 8;');
    await ui.run('openPackageConfig("build-1", null)');
    await flush();
    assert.equal(ui.element('packageConfigModeLabel').textContent, 'Saved runtime mode');
    assert.equal(ui.element('packageConfigModeValue').textContent, 'LIVE');
    assert.equal(ui.element('packageConfigModeValue').attrs.get('data-mode'), 'live');
    assert.equal(ui.element('packageConfigMode').hidden, false);
    assert.match(ui.element('packageConfigModeHelp').textContent, /saved build setting overrides hyperbricks.mode/);
    assert.equal(editor.current().getValue(), source);
    assert.equal(ui.element('savePackageConfig').disabled, true);

    // A save response may reflect a build-mode change made by another client.
    packageConfig.runtime_mode = 'development';
    const edited = source + 'free: "001"\r\n';
    editor.current().userChange(edited);
    editor.current().save();
    await flush();
    assert.equal(ui.element('packageConfigModeValue').textContent, 'DEVELOPMENT');
    assert.equal(ui.element('packageConfigModeValue').attrs.get('data-mode'), 'development');
    assert.equal(editor.current().getValue(), edited);
    const put = ui.requests.find(request => request.method === 'PUT' && new URL(request.url).pathname.endsWith('/package-config'));
    assert.equal(JSON.parse(put.body).content, edited);
  }
});

test('the source editor explains its fixed Development runtime even when YAML says Live', async () => {
  const source = 'hyperbricks:\n  mode: live\n';
  const editor = createEditorHarness();
  const ui = setup({
    mode: 'local', editorModule: editor.module,
    packageConfig: {content: source, sha256: 'sha', scope: 'source', runtime_mode: 'development', restart_required: false}
  });
  await flush();
  ui.run('state.selectedModule = "demo"; state.selectionToken = 9;');
  await ui.run('openPackageConfig("dev", null)');
  await flush();
  assert.equal(ui.element('packageConfigModeLabel').textContent, 'Runtime mode');
  assert.equal(ui.element('packageConfigModeValue').textContent, 'DEVELOPMENT');
  assert.match(ui.element('packageConfigModeHelp').textContent, /always runs in Development, regardless of hyperbricks.mode/);
  assert.equal(editor.current().getValue(), source);
  assert.equal(ui.element('savePackageConfig').disabled, true);
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

test('duplicate action is available only for HRA builds and disables while that build is pending', () => {
  for (const mode of ['local', 'remote']) {
    const ui = setup({mode});
    ui.run(`
      testRows = [];
      document.createElement = () => ({innerHTML: "", classList: {add() {}}, querySelectorAll: () => []});
      ui.buildRows.appendChild = row => testRows.push(row);
      wireRowMenus = () => {};
      state.selectedModule = "demo";
      state.builds = [
        {build_id: "dev", format: "dev", is_dev: true},
        {build_id: "zip-1", format: "zip"},
        {build_id: "hra-1", format: "hra"}
      ];
      renderBuilds();
    `);
    assert.doesNotMatch(ui.run('testRows[0].innerHTML'), /Duplicate as new build/);
    assert.doesNotMatch(ui.run('testRows[2].innerHTML'), /Duplicate as new build/);
    assert.match(ui.run('testRows[4].innerHTML'), /data-action="duplicate"[^>]*>.*Duplicate as new build/s);
    ui.run('state.duplicateBuilds.add(state.selectedModule + String.fromCharCode(0) + "hra-1"); testRows = []; renderBuilds();');
    assert.match(ui.run('testRows[4].innerHTML'), /data-action="duplicate" disabled/);
  }
});

test('duplicating a current HRA runtime creates a new build without starting it in both interfaces', async () => {
  for (const mode of ['local', 'remote']) {
    let releaseDuplicate;
    const duplicateGate = new Promise(resolve => { releaseDuplicate = resolve; });
    const sourceBuild = {build_id: 'build-1', format: 'hra', runtime_mode: 'live'};
    const ui = setup({
      mode,
      saved: 'test-secret',
      responseForRequest: async url => {
        if (url.pathname.endsWith('/duplicate')) {
          await duplicateGate;
          return {build_id: 'new-build'};
        }
        if (url.pathname.endsWith('/builds')) {
          return {versions: [sourceBuild, {build_id: 'new-build', format: 'hra', runtime_mode: 'live'}]};
        }
      }
    });
    await flush();
    ui.run(`
      renderBuilds = () => {};
      state.selectedModule = "module name";
      state.selectionToken = 7;
      state.builds = [{build_id: "build-1", format: "hra", runtime_mode: "live"}];
    `);

    const duplicating = ui.run('duplicateBuild("build-1")');
    await flush();
    assert.equal(ui.run('state.duplicateBuilds.size'), 1);
    await ui.run('duplicateBuild("build-1")');
    assert.equal(ui.requests.filter(request => new URL(request.url).pathname.endsWith('/duplicate')).length, 1);

    releaseDuplicate();
    await duplicating;
    const post = ui.requests.find(request => new URL(request.url).pathname.endsWith('/duplicate'));
    assert.equal(new URL(post.url).pathname, (mode === 'local' ? '/local' : '/deploy') + '/modules/module%20name/builds/build-1/duplicate');
    assert.equal(post.method, 'POST');
    assert.equal(post.body, undefined);
    assert.equal(ui.requests.filter(request => new URL(request.url).pathname.endsWith('/builds')).length, 1);
    assert.equal(ui.requests.filter(request => /\/activate|\/restart|\/stop/.test(new URL(request.url).pathname)).length, 0);
    assert.equal(ui.run('state.builds.some(build => build.build_id === "new-build")'), true);
    assert.equal(ui.run('state.duplicateBuilds.size'), 0);
    assert.match(ui.element('activity').textContent, /Created build new-build from build-1\. It has not been started\./);
  }
});

test('duplicate errors keep the source available and re-enable the action', async () => {
  const ui = setup({
    saved: 'test-secret',
    responseForRequest: url => url.pathname.endsWith('/duplicate')
      ? {status: 409, error: 'runtime changed while duplicating'}
      : undefined
  });
  await flush();
  ui.run(`
    renderBuilds = () => {};
    state.selectedModule = "demo";
    state.selectionToken = 7;
    state.builds = [{build_id: "build-1", format: "hra"}];
  `);
  await ui.run('duplicateBuild("build-1").catch(handleError)');
  assert.equal(ui.run('state.duplicateBuilds.size'), 0);
  assert.equal(ui.run('state.builds.length'), 1);
  assert.match(ui.element('activity').textContent, /runtime changed while duplicating/);
  assert.equal(ui.requests.filter(request => new URL(request.url).pathname.endsWith('/builds')).length, 0);
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
