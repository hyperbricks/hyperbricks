import {fieldGroups, planRecovery, resolveRecovery} from './recovery.mjs';
import {readNavigation, resolveNavigation, selectionURL, editPageURL} from './navigation.mjs';
import {readResponse} from './http.mjs';

(() => {
  'use strict';
  const base = document.querySelector('meta[name="spaces-base"]').content;
  const $ = (s, root = document) => root.querySelector(s);
  const state = { snapshot: null, selected: null, trash: false, tab: 'content', dirty: false, busy: false, meta: {}, resets: new Set(), files: new Map() };
  const icons = () => window.lucide?.createIcons();
  const documentState = {doc: null, name: null, field: null, dirty: false, busy: false, conflict: null};
  const drawer = $('#spaces-drawer'), sidebar = $('aside'), narrow = matchMedia('(max-width: 850px)');
  let recovery = null, imageObjectURL = null, acceptedURL = location.href, highlightedField = null, highlightTimer;
  function closeDrawer() { if (drawer.open) drawer.close(); }
  function placeSidebar() { closeDrawer(); (narrow.matches ? $('#drawer-body') : $('#sidebar-slot')).append(sidebar); }
  $('#drawer-open').onclick = () => { drawer.showModal(); $('#drawer-open').setAttribute('aria-expanded', 'true'); };
  $('#drawer-close').onclick = closeDrawer;
  drawer.addEventListener('close', () => $('#drawer-open').setAttribute('aria-expanded', 'false'));
  drawer.addEventListener('click', event => { if (event.target === drawer) closeDrawer(); });
  drawer.addEventListener('keydown', event => {
    if (event.key !== 'Tab') return;
    const controls = [...drawer.querySelectorAll('button,input')].filter(node => !node.disabled && node.getClientRects().length);
    const first = controls[0], last = controls.at(-1);
    if (first && event.shiftKey && document.activeElement === first) {event.preventDefault(); last.focus();}
    else if (last && !event.shiftKey && document.activeElement === last) {event.preventDefault(); first.focus();}
  });
  narrow.addEventListener('change', placeSidebar); placeSidebar();
  function el(tag, attrs = {}, ...children) {
    const node = document.createElement(tag);
    for (const [key, value] of Object.entries(attrs)) {
      if (key.startsWith('on')) node.addEventListener(key.slice(2), value);
      else if (key === 'class') node.className = value;
      else if (key === 'text') node.textContent = value;
      else if (key in node && key !== 'form') node[key] = value;
      else node.setAttribute(key, value);
    }
    children.flat().forEach(child => { if (child !== null && child !== undefined) node.append(child instanceof Node ? child : document.createTextNode(String(child))); });
    return node;
  }
  const icon = name => el('i', { 'data-lucide': name });
  const tool = (name, label, action, extra = {}) => el('button', { type: 'button', class: 'btn btn-sm btn-square btn-ghost', title: label, 'aria-label': label, onclick: action, ...extra }, icon(name));
  function notice(text = '', error = false) {
    $('#notice').textContent = text;
    $('#notice').className = error ? 'error alert alert-error' : text ? 'alert alert-success' : '';
  }
  async function api(path = '/api', options = {}) {
    const response = await fetch(base + path, { ...options, headers: { 'X-Spaces-Request': '1', ...options.headers } });
    return readResponse(response);
  }
  async function post(mutation) { return api('/api', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(mutation) }); }
  function mayLeave() { return !state.busy && !documentState.busy && (!state.dirty || window.confirm('Discard unsaved changes?')) && (!documentState.dirty || window.confirm('Discard unsaved document changes?')); }
  function dirty() { state.dirty = true; updateSaveState(); }
  function updateSaveState() {
    const label = $('.save-state');
    if (label) {
      label.textContent = state.loading ? 'Loading...' : state.busy ? 'Saving...' : state.dirty ? 'Unsaved changes' : 'Saved to source';
      label.classList.toggle('dirty', state.dirty);
    }
    const save = $('#save'); if (save) save.disabled = state.busy || !state.dirty || !state.snapshot.write;
    const discard = $('#discard'); if (discard) discard.disabled = state.busy || !state.dirty;
    $('#new').disabled = state.busy || !state.snapshot?.write || !state.snapshot.sources.length;
    $('#reload').disabled = state.busy;
  }
  function selected() { return state.snapshot?.spaces.find(s => s.name === state.selected && s.trashed === state.trash); }
  function initDraft() {
    state.meta = structuredClone(selected()?.meta || {});
    state.resets = new Set(); state.files.clear(); state.dirty = false;
  }
  function syncSelectionURL(name, field = null) {
    acceptedURL = selectionURL(location.href, name, field);
    history.replaceState(history.state, '', acceptedURL);
  }
  function contentTab() {
    state.tab = 'content';
    if (!$('#content-panel')) return;
    $('#content-panel').hidden = false; $('#metadata-panel').hidden = true;
    for (const button of $('.editor-tabs').children) button.setAttribute('aria-selected', String(button.textContent === 'Content'));
  }
  function clearFieldHighlight() {
    clearTimeout(highlightTimer);
    highlightedField?.classList.remove('field-target'); highlightedField = null;
  }
  function showNavigation(result) {
    clearFieldHighlight();
    if (!result) return;
    if (result.error) {
      notice(result.error, true);
      $('#notice').scrollIntoView({block: 'nearest'});
      return;
    }
    notice();
    if (!result.field) return;
    contentTab();
    const input = document.getElementById(result.field.id);
    if (!input) { notice('This field could not be opened. Reload the source files and try the edit link again.', true); return; }
    highlightedField = input.closest('.field'); highlightedField.classList.add('field-target');
    notice(`Editing ${result.field.label} in ${result.space.title || result.space.name}.`);
    requestAnimationFrame(() => {
      if (!input.isConnected) return;
      input.focus({preventScroll: true});
      input.closest('.field').scrollIntoView({block: 'center', behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth'});
    });
    highlightTimer = setTimeout(clearFieldHighlight, 3000);
  }
  function navigateFromLocation() {
    const result = resolveNavigation(state.snapshot.spaces, readNavigation(location.search));
    const name = result ? result.space?.name || null : state.snapshot.spaces.find(s => !s.trashed)?.name || null;
    const sameSpace = name === state.selected && !state.trash;
    if (state.busy || documentState.busy || (!sameSpace && !mayLeave()) || (sameSpace && documentState.dirty && !window.confirm('Discard unsaved document changes?'))) {
      history.replaceState(history.state, '', acceptedURL); return;
    }
    documentState.dirty = false;
    document.querySelectorAll('dialog[open]').forEach(dialog => dialog.close());
    acceptedURL = location.href;
    if (!sameSpace) {
      state.selected = name; state.trash = false; state.tab = 'content'; initDraft(); render();
    } else contentTab();
    showNavigation(result);
  }
  async function reload(keep = true) {
    const controls = [...$('#main').querySelectorAll('input,textarea,select,button')];
    const disabled = controls.map(c => c.disabled);
    state.busy = true; state.loading = true; updateSaveState();
    controls.forEach(c => { c.disabled = true; });
    try {
      const snapshot = await api(); state.snapshot = snapshot;
      const spaces = snapshot.spaces.filter(s => s.trashed === state.trash);
      const target = state.trash ? null : resolveNavigation(snapshot.spaces, readNavigation(location.search));
      if (target) { state.selected = target.space?.name || null; state.tab = 'content'; }
      else if (!keep || !spaces.some(s => s.name === state.selected)) state.selected = spaces[0]?.name || null;
      initDraft(); state.busy = false; state.loading = false; render();
      if (target) showNavigation(target);
      else syncSelectionURL(state.trash ? null : state.selected);
    } finally {
      state.busy = false; state.loading = false;
      controls.forEach((c,i) => { c.disabled = disabled[i]; });
      updateSaveState();
    }
  }
  function renderList() {
    const search = $('#search').value.toLowerCase();
    $('#active-count').textContent = state.snapshot.spaces.filter(s => !s.trashed).length;
    $('#trash-count').textContent = state.snapshot.spaces.filter(s => s.trashed).length;
    $('#active-tab').setAttribute('aria-selected', String(!state.trash));
    $('#trash-tab').setAttribute('aria-selected', String(state.trash));
    const spaces = state.snapshot.spaces.filter(s => s.trashed === state.trash && `${s.name} ${s.title} ${s.route} ${s.source}`.toLowerCase().includes(search));
    $('#list').replaceChildren(...spaces.map(s => el('button', { class: 'space-item', 'aria-current': String(s.name === state.selected), onclick: () => {
      if (s.name === state.selected) { syncSelectionURL(s.trashed ? null : s.name); clearFieldHighlight(); notice(); closeDrawer(); return; }
      if (!mayLeave()) return;
      state.selected = s.name; state.tab = 'content'; syncSelectionURL(s.trashed ? null : s.name); clearFieldHighlight(); notice(); initDraft(); render(); closeDrawer();
    } }, icon(s.trashed ? 'archive' : 'file-text'), el('span', {}, el('strong', {}, s.title || s.name), el('small', {}, s.name), el('small', {}, '/' + (s.route === 'index' ? '' : s.route))))));
    if (!spaces.length) $('#list').append(el('p', { class: 'no-items' }, search ? 'No matching Spaces' : state.trash ? 'Trash is empty' : 'No Spaces yet'));
    icons();
  }
  function pageURL(route) { return '/' + (route === 'index' ? '' : route.replace(/^\/+/, '')); }
  function render(draftValues = {}) {
    $('#access').textContent = state.snapshot.write ? 'Read / write' : 'Read only';
    renderList();
    const s = selected();
    if (!s) {
      $('#main').replaceChildren(el('div', { class: 'empty' }, icon(state.trash ? 'archive' : 'layers'), el('h2', {}, state.trash ? 'Trash is empty' : 'No Space selected'), !state.trash ? el('button', { class: 'btn btn-sm btn-primary', disabled: !state.snapshot.write || !state.snapshot.sources.length, onclick: openCreate }, icon('plus'), 'Create Space') : null));
      updateSaveState(); icons(); return;
    }
    const actions = el('div', { class: 'heading-actions' });
    if (!s.trashed) actions.append(el('a', { class: 'btn btn-sm btn-square btn-ghost open-link', href: pageURL(s.route), target: '_blank', rel: 'noopener', title: 'Open page', 'aria-label': 'Open page' }, icon('external-link')), el('a', { class: 'btn btn-sm btn-ghost edit-page-link', href: editPageURL(s.route, location.href), target: '_blank', rel: 'noopener', title: 'Open page with contextual edit links', 'aria-label': 'Edit page' }, icon('mouse-pointer-2'), el('span', {}, 'Edit page')), tool('archive', 'Move to Trash', trashSpace, { disabled: !state.snapshot.write || !!s.trash_blocked, title: s.trash_blocked || 'Move to Trash' }));
    const heading = el('div', { class: 'editor-heading' }, el('div', {}, el('div', { class: 'breadcrumb' }, s.source + ' / ' + s.name), el('h2', {}, s.title || s.name)), actions);
    $('#main').replaceChildren(heading);
    if (s.trashed) {
      $('#main').append(el('section', { class: 'trash-panel' }, el('h3', {}, 'In Trash'), el('dl', {}, el('dt', {}, 'Route'), el('dd', {}, pageURL(s.route)), el('dt', {}, 'Source'), el('dd', {}, s.source), el('dt', {}, 'File'), el('dd', {}, s.file)), s.trash_blocked ? el('p', { class: 'blocked' }, s.trash_blocked) : null, el('button', { class: 'btn btn-sm btn-primary', disabled: !state.snapshot.write || !!s.trash_blocked, onclick: restoreSpace }, icon('archive-restore'), 'Restore Space')));
      updateSaveState(); icons(); return;
    }
    const tabs = el('div', { class: 'tabs tabs-border editor-tabs', role: 'tablist', 'aria-label': 'Space editor' });
    for (const [key, title] of [['content', 'Content'], ['metadata', 'Metadata']]) tabs.append(el('button', { class: 'tab', role: 'tab', 'aria-selected': String(state.tab === key), onclick: () => {
      state.tab = key;
      for (const button of tabs.children) button.setAttribute('aria-selected', String(button.textContent === title));
      $('#content-panel').hidden = key !== 'content'; $('#metadata-panel').hidden = key !== 'metadata';
    } }, title));
    $('#main').append(tabs);
    const form = el('form', { id: 'edit-form', onsubmit: save });
    form.addEventListener('invalid', event => {
      if (event.target.closest('#content-panel')) {
        state.tab = 'content'; $('#content-panel').hidden = false; $('#metadata-panel').hidden = true;
        for (const button of tabs.children) button.setAttribute('aria-selected', String(button.textContent === 'Content'));
      }
    }, true);
    const body = el('div', { class: 'editor-body' });
    const content = el('section', { id: 'content-panel', class: 'hb-surface', hidden: state.tab !== 'content' });
    content.append(el('div', { class: 'identity' }, el('label', {}, 'Title', el('input', { class: 'input', name: 'title', value: s.title, required: true, maxLength: 200, readOnly: !state.snapshot.write, oninput: dirty })), el('label', {}, 'Route', el('input', { class: 'input', name: 'route', value: s.route, required: true, readOnly: !state.snapshot.write, oninput: dirty }))));
    content.append(el('div', { class: 'section-heading' }, el('h3', {}, 'Content', el('small', {}, `${s.fields.length} fields`))));
    if (!s.fields.length) content.append(el('p', { class: 'subtle' }, 'No editable content fields'));
    for (const group of fieldGroups(s.fields)) {
      const section = el('div', {class: 'field-group'});
      if (group.name) section.append(el('h3', {class: 'field-group-title'}, group.name));
      group.fields.forEach(f => section.append(fieldControl(Object.hasOwn(draftValues, f.id) ? {...f, value: draftValues[f.id]} : f))); content.append(section);
    }
    const metadata = el('section', { id: 'metadata-panel', class: 'hb-surface', hidden: state.tab !== 'metadata' });
    metadata.append(el('div', { class: 'section-heading' }, el('h3', {}, 'Document Metadata'), tool('plus', 'Add metadata', () => { $('#meta-form').reset(); $('.form-error', $('#meta-form')).textContent = ''; $('#meta-dialog').showModal(); }, { disabled: !state.snapshot.write })), el('div', { class: 'sharing-warning', id: 'sharing-warning' }), el('div', { class: 'meta-list', id: 'meta-list' }), el('div', { class: 'removed-list', id: 'removed-list' }));
    body.append(content, metadata); form.append(body);
    form.append(el('footer', { class: 'savebar' }, el('span', { class: 'save-state', role: 'status' }), el('button', { type: 'button', class: 'btn btn-sm btn-ghost', id: 'discard', onclick: () => { if (mayLeave()) { initDraft(); render(); } } }, 'Discard'), el('button', { class: 'btn btn-sm btn-primary', id: 'save', type: 'submit' }, icon('save'), 'Save Changes')));
    form.append(el('div', {class: 'upload-status', id: 'upload-status', hidden: true}, el('label', {htmlFor: 'upload-progress', id: 'upload-label'}, 'Uploading'), el('progress', {id: 'upload-progress', max: 100, value: 0})));
    $('#main').append(form); renderMeta(); updateSaveState(); icons();
  }
  function fieldControl(f) {
    const input = el(f.type === 'textarea' ? 'textarea' : 'input', { class: f.type === 'textarea' ? 'textarea' : 'input', id: f.id, name: f.id, value: f.value, placeholder: f.placeholder || '', required: f.required, readOnly: !state.snapshot.write, oninput: dirty });
    if (f.type === 'textarea') input.rows = f.rows || 3;
    else input.type = f.type === 'email' ? 'email' : 'text';
    // Server limits count Unicode code points, not the browser's UTF-16 maxlength.
    const validateLength = () => input.setCustomValidity(f.max && Array.from(input.value).length > f.max ? `${f.label} exceeds ${f.max} characters.` : '');
    input.addEventListener('input', validateLength); validateLength();
    const ownerPath = f.path.slice(0, -1);
    if (ownerPath.at(-1) === 'values') ownerPath.pop();
    const field = el('div', { class: 'field' }, el('div', { class: 'field-heading' }, el('label', { htmlFor: f.id }, f.label, f.required ? el('span', { class: 'required' }, ' *') : null), el('span', { class: 'path' }, ownerPath.join('.'))));
    if (f.type === 'asset') {
      input.required = false;
      const preview = el('div', { class: 'asset-preview' });
      const showPreview = (url = input.value, filename = '') => {
        preview.replaceChildren();
        if (/\.(png|jpe?g|webp|gif)(\?|$)/i.test(url) || url.startsWith('blob:')) preview.append(el('button', {type: 'button', class: 'image-thumb', title: 'Preview ' + f.label, 'aria-label': 'Preview ' + f.label, onclick: () => openImage(input.value, f.label, state.files.get(f.id), assetPreview(f, input.value))}, el('img', { src: assetPreview(f, url), alt: f.label })));
        if (filename || input.value) preview.append(el('span', {}, filename || input.value));
      };
      input.addEventListener('input', () => { state.files.delete(f.id); showPreview(); });
      const controls = el('div', { class: 'asset-input' }, input, tool('folder-open', 'Select existing asset', () => openAssets(f.id, ref => { input.value = ref; state.files.delete(f.id); dirty(); showPreview(); }), { disabled: !state.snapshot.write }));
      if (f.upload) {
        const file = el('input', { class: 'file-control', type: 'file', accept: f.upload.accept.join(','), 'aria-label': `Upload ${f.label}`, onchange: () => {
          const chosen = file.files[0]; if (!chosen) return;
          if (chosen.size > f.upload.max_bytes) { notice(`${f.label}: upload exceeds ${Math.round(f.upload.max_bytes / 1024)} KiB.`, true); file.value = ''; return; }
          state.files.set(f.id, chosen); dirty();
          const url = chosen.type.startsWith('image/') ? URL.createObjectURL(chosen) : '';
          showPreview(url, chosen.name + ` (${Math.ceil(chosen.size / 1024)} KiB, pending save)`);
          if (url) preview.querySelector('img').onload = () => URL.revokeObjectURL(url);
        } });
        controls.append(file, tool('upload', 'Upload file', () => file.click(), { disabled: !state.snapshot.write }));
      }
      if (f.edit) controls.append(tool('file-pen-line', 'Edit ' + f.label, () => openDocument(f), {disabled: !state.snapshot.write || !f.value}));
      controls.append(tool('x', 'Clear asset', () => { input.value = ''; state.files.delete(f.id); dirty(); showPreview(); }, { disabled: !state.snapshot.write || f.required }));
      field.append(controls, preview); showPreview();
      const pending = state.files.get(f.id);
      if (pending) {
        const url = pending.type.startsWith('image/') ? URL.createObjectURL(pending) : '';
        showPreview(url, `${pending.name} (${Math.ceil(pending.size / 1024)} KiB, pending save)`);
        if (url) preview.querySelector('img').onload = () => URL.revokeObjectURL(url);
      }
    } else field.append(input);
    if (f.help || f.max) {
      const helpID = 'field-help:' + f.id;
      input.setAttribute('aria-describedby', helpID);
      field.append(el('small', {id: helpID}, [f.help, f.max ? `Maximum ${f.max} characters` : ''].filter(Boolean).join(' / ')));
    }
    return field;
  }
  function captureMeta() {
    const meta = { ...state.meta };
    for (const row of $('#meta-list')?.children || []) {
      const original = row.dataset.original;
      const key = $('.meta-key input', row).value.trim();
      const value = $('.meta-value input, .meta-value textarea, .meta-value select', row).value;
      if (!key) throw new Error('Metadata keys cannot be empty.');
      if (key !== original) { if (Object.hasOwn(meta, key) && meta[key] !== null) throw new Error(`Duplicate metadata key: ${key}`); meta[original] = null; }
      meta[key] = value;
    }
    return meta;
  }
  function syncMeta() { try { state.meta = captureMeta(); return true; } catch (error) { notice(error.message, true); return false; } }
  function renderMeta() {
    const s = selected();
    const rows = Object.entries(state.meta).filter(([,v]) => v !== null).sort(([a],[b]) => a.localeCompare(b)).map(([key, value]) => {
      const multiline = key === 'description' || key === 'og:description' || value.includes('\n');
      const input = el(multiline ? 'textarea' : 'input', { class: multiline ? 'textarea' : 'input', value, rows: 3, readOnly: !state.snapshot.write, 'aria-label': `${key} value`, oninput: () => { state.resets.delete(key); dirty(); } });
      if (!multiline && (key === 'og:url' || key === 'og:image')) input.type = 'url';
      const keyInput = el('input', { class: 'input', value: key, readOnly: !state.snapshot.write, 'aria-label': `${key} key`, oninput: () => { state.resets.delete(key); dirty(); } });
      const clearRenamedDraft = () => {
        const renamed = keyInput.value.trim();
        if (renamed !== key) { delete state.meta[renamed]; state.resets.delete(renamed); }
      };
      const sourceValue = s.source_meta[key];
      const cell = el('div', { class: 'meta-value' }, input);
      if (key === 'og:type') {
        const types = [...new Set(['website', 'article', 'profile', value])];
        const select = el('select', { class: 'select','aria-label': 'og:type value', disabled: !state.snapshot.write, onchange: () => {state.resets.delete(key); dirty();}}, types.map(type => el('option', {value: type, selected: type === value}, type)));
        cell.replaceChildren(select);
      }
      if (key === 'og:image' && state.snapshot.sharing_image && state.snapshot.public_origin) {
        const file = el('input', { class: 'file-control', type: 'file', accept: state.snapshot.sharing_image.accept.join(','), 'aria-label': 'Upload sharing image', onchange: () => { if (file.files[0]) { state.files.set('@meta.og:image', file.files[0]); dirty(); notice('Sharing image selected. Save Changes to upload.'); } } });
        cell.append(el('div', { class: 'asset-input' }, tool('folder-open', 'Select sharing image', () => openAssets('@meta.og:image', ref => { input.value = ref; state.files.delete('@meta.og:image'); dirty(); }), { disabled: !state.snapshot.write }), tool('upload', 'Upload sharing image', () => file.click(), { disabled: !state.snapshot.write }), file));
      }
      return el('div', { class: 'meta-row', 'data-original': key }, el('div', { class: 'meta-key' }, keyInput, el('small', {}, sourceValue === value ? 'Source' : 'Override')), cell, el('div', { class: 'meta-actions' }, tool('rotate-ccw', 'Reset to source', () => { if (!syncMeta()) return; clearRenamedDraft(); state.resets.add(key); if (Object.hasOwn(s.source_meta, key)) state.meta[key] = s.source_meta[key]; else delete state.meta[key]; dirty(); renderMeta(); }, { disabled: !state.snapshot.write }), tool('minus', 'Remove metadata', () => { if (!syncMeta()) return; clearRenamedDraft(); state.meta[key] = null; state.resets.delete(key); if (key === 'og:image') state.files.delete('@meta.og:image'); dirty(); renderMeta(); }, { disabled: !state.snapshot.write })));
    });
    $('#meta-list').replaceChildren(...rows);
    const removed = Object.entries(state.meta).filter(([,v]) => v === null);
    $('#removed-list').replaceChildren(...(removed.length ? [el('h3', {}, 'Removed For This Space'), ...removed.map(([key]) => el('div', { class: 'removed-entry' }, el('code', {}, key), tool('plus', 'Re-add ' + key, () => { if (!syncMeta()) return; state.meta[key] = s.source_meta[key] || ''; state.resets.delete(key); dirty(); renderMeta(); }, { disabled: !state.snapshot.write }), tool('rotate-ccw', 'Reset ' + key + ' to source', () => { if (!syncMeta()) return; state.resets.add(key); if (Object.hasOwn(s.source_meta, key)) state.meta[key] = s.source_meta[key]; else delete state.meta[key]; dirty(); renderMeta(); }, { disabled: !state.snapshot.write })))] : []));
    const missing = ['og:title', 'og:type', 'og:image', 'og:url'].filter(k => !state.meta[k]);
    $('#sharing-warning').textContent = Object.entries(state.meta).some(([k,v]) => k.startsWith('og:') && v !== null) && missing.length ? 'Sharing metadata missing: ' + missing.join(', ') : '';
    icons();
  }
  function mutation() {
    const s = selected(), form = $('#edit-form');
    const values = {}; s.fields.forEach(f => { values[f.id] = document.getElementById(f.id).value; });
    const current = captureMeta(), meta = {};
    for (const [key, value] of Object.entries(current)) {
      if (!Object.hasOwn(s.meta, key) || s.meta[key] !== value) meta[key] = value;
    }
    state.resets.forEach(k => delete meta[k]);
    return { action: 'save', revision: state.snapshot.revision, name: s.name, title: form.elements.title.value, route: form.elements.route.value, values, meta, reset_meta: [...state.resets] };
  }
  async function save(event) {
    event.preventDefault(); if (state.busy || !state.snapshot.write) return;
    let m;
    try { m = mutation(); } catch (error) { notice(error.message, true); return; }
    state.busy = true; updateSaveState(); notice();
    const before = structuredClone(selected());
    // Freeze editable controls while an in-flight response is being applied.
    const controls = [...$('#edit-form').querySelectorAll('input,textarea,select,button')];
    const disabled = controls.map(c => c.disabled); controls.forEach(c => { c.disabled = true; });
    let savedUploads = 0;
    try {
      if (state.files.size) {
        for (const [field, file] of [...state.files]) {
          const body = new FormData(); body.set('mutation', JSON.stringify(m)); body.set('field', field); body.set('file', file);
          await uploadFile(body, file.name);
          savedUploads++;
          state.files.delete(field);
          const snapshot = await api();
          state.snapshot = snapshot; m.revision = snapshot.revision;
          const updated = snapshot.spaces.find(s => s.name === m.name && !s.trashed);
          if (field === '@meta.og:image') {
            m.meta['og:image'] = updated.meta['og:image'];
            const row = [...$('#meta-list').children].find(row => row.dataset.original === 'og:image');
            if (row) $('.meta-value input, .meta-value textarea, .meta-value select', row).value = updated.meta['og:image'];
          } else {
            m.values[field] = updated.fields.find(f => f.id === field).value;
            document.getElementById(field).value = m.values[field];
          }
        }
      } else await post(m);
      state.busy = false; await reload();
      notice(state.snapshot.watch ? 'Saved to source.' : 'Saved to source. Runtime watching is disabled.');
    } catch (error) {
      state.busy = false; controls.forEach((c,i) => { c.disabled = disabled[i]; });
      notice((savedUploads ? `${savedUploads} upload(s) saved. ` : '') + error.message, true); updateSaveState();
      if (error.status === 409) $('#notice').append(el('button', {type: 'button', class: 'btn btn-sm btn-ghost', onclick: () => openRecovery(before)}, icon('git-compare-arrows'), 'Review Changes'));
    } finally {
      if ($('#upload-status')) $('#upload-status').hidden = true;
    }
  }
  async function openAssets(field, pick) {
    $('#asset-list').replaceChildren(el('p', { class: 'subtle' }, 'Loading assets...')); $('#asset-dialog').showModal();
    try {
      const assets = await api(`/api/assets?name=${encodeURIComponent(state.selected)}&field=${encodeURIComponent(field)}`);
      $('#asset-list').replaceChildren(...assets.map(asset => el('button', { class: 'asset-option', onclick: () => { pick(asset.reference); $('#asset-dialog').close(); } }, asset.image ? el('img', { src: asset.preview || asset.reference, alt: asset.name, loading: 'lazy' }) : icon('file-text'), el('span', {}, asset.name), el('small', {}, Math.ceil(asset.size / 1024) + ' KiB'))));
      if (!assets.length) $('#asset-list').append(el('p', { class: 'subtle' }, 'No permitted assets')); icons();
    } catch (error) { $('#asset-list').replaceChildren(el('p', { class: 'form-error hb-form-error' }, error.message)); }
  }
  function uploadFile(body, filename) {
    $('#upload-status').hidden = false; $('#upload-label').textContent = 'Uploading ' + filename;
    $('#upload-progress').value = 0;
    return new Promise((resolve, reject) => {
      const request = new XMLHttpRequest();
      request.open('POST', base + '/api/upload'); request.setRequestHeader('X-Spaces-Request', '1');
      request.upload.onprogress = event => {
        if (event.lengthComputable) $('#upload-progress').value = Math.round(event.loaded / event.total * 100);
        if (event.loaded === event.total) $('#upload-label').textContent = 'Saving ' + filename;
      };
      request.onload = () => {
        let response; try { response = JSON.parse(request.responseText); } catch { reject(new Error('Invalid upload response. Reload source files before retrying.')); return; }
        if (request.status >= 200 && request.status < 300) resolve(response);
        else { const error = new Error(response.error || 'Upload failed'); error.status = request.status; reject(error); }
      };
      request.onerror = () => reject(new Error('Upload connection failed. Reload source files before retrying.'));
      request.send(body);
    });
  }
  function assetPreview(field, reference) {
    const directory = field.upload?.directory || field.directory;
    if (reference.startsWith('blob:') || directory?.base !== 'resources') return reference;
    return `${base}/api/asset?${new URLSearchParams({name: state.selected, field: field.id, reference})}`;
  }
  async function openImage(reference, label, file, previewURL = reference) {
    if (imageObjectURL) URL.revokeObjectURL(imageObjectURL);
    imageObjectURL = file ? URL.createObjectURL(file) : null;
    const url = imageObjectURL || previewURL;
    const filename = file?.name || reference.split('/').pop()?.split('?')[0] || label;
    $('#image-title').textContent = filename; $('#image-error').textContent = '';
    $('#image-details').textContent = 'Loading image...'; setImageZoom(false);
    const img = el('img', {src: url, alt: label}); let size = file?.size;
    const details = () => {
      if ($('#image-stage img') === img && img.naturalWidth) $('#image-details').textContent = `${img.naturalWidth} x ${img.naturalHeight} px` + (size !== undefined ? ` / ${Math.ceil(size / 1024)} KiB` : '');
    };
    img.onload = details; img.onerror = () => {$('#image-error').textContent = 'Image could not be loaded.'; $('#image-details').textContent = '';};
    $('#image-stage').replaceChildren(img); $('#image-dialog').showModal();
    if (!file && new URL(url, location.href).origin === location.origin) {
      try {const response = await fetch(url, {method: 'HEAD'}); const length = response.headers.get('Content-Length'); if (response.ok && length) {size = Number(length); details();}} catch { /* The image remains viewable without size metadata. */ }
    }
  }
  function setImageZoom(original) {
    $('#image-stage').classList.toggle('original', original);
    $('#image-fit').setAttribute('aria-pressed', String(!original)); $('#image-original').setAttribute('aria-pressed', String(original));
  }
  $('#image-fit').onclick = () => setImageZoom(false); $('#image-original').onclick = () => setImageZoom(true);
  $('#image-dialog').addEventListener('close', () => {if (imageObjectURL) URL.revokeObjectURL(imageObjectURL); imageObjectURL = null; $('#image-stage').replaceChildren();});

  function updateDocument() {
    const d = documentState, doc = d.doc;
    $('#document-state').textContent = d.busy ? 'Working...' : d.dirty ? 'Unsaved document' : doc ? 'Saved document' : '';
    $('#document-content').disabled = d.busy || !doc;
    $('#document-save').disabled = d.busy || !doc || !d.dirty || !!d.conflict;
    $('#document-copy').disabled = d.busy || !doc || !!d.conflict;
    $('#document-preview').disabled = d.busy || !doc || !!d.conflict;
    $('#document-write').disabled = d.busy; $('#document-close').disabled = d.busy;
    $('#document-save span').textContent = doc?.shared ? 'Save Shared Document' : 'Save Document';
    if (doc) {
      $('#document-info').textContent = `${doc.reference} / ${new TextEncoder().encode($('#document-content').value).length} of ${doc.max_bytes} bytes`;
      $('#document-use-count').textContent = `(${doc.usages.length})`;
      $('#document-usages').replaceChildren(...doc.usages.map(use => el('li', {}, el('strong', {}, use.title || use.name), ` / ${use.label} / ${use.trashed ? 'Trash' : use.kind === 'source' ? 'Source' : 'Space'}`, el('small', {}, use.name))));
    }
  }
  function documentView(preview) {
    $('#document-content').hidden = preview; $('#document-rendered').hidden = !preview;
    $('#document-write').setAttribute('aria-selected', String(!preview)); $('#document-preview').setAttribute('aria-selected', String(preview));
  }
  async function openDocument(field) {
    if (state.dirty || state.files.size) {notice('Save or discard Space changes before editing its document.', true); return;}
    if (state.busy) return;
    Object.assign(documentState, {doc: null, name: state.selected, field: field.id, dirty: false, busy: true, conflict: null});
    $('#document-content').value = ''; $('#document-info').textContent = 'Loading document...'; $('#document-error').textContent = '';
    $('#document-usages').replaceChildren(); $('#document-use-count').textContent = ''; $('#document-conflict').hidden = true;
    documentView(false); updateDocument(); $('#document-dialog').showModal();
    try {
      const doc = await api(`/api/document?name=${encodeURIComponent(state.selected)}&field=${encodeURIComponent(field.id)}`);
      documentState.doc = doc; $('#document-content').value = doc.content;
    } catch(error) {$('#document-error').textContent = error.message;}
    finally {documentState.busy = false; updateDocument();}
  }
  function closeDocument() {
    if (documentState.busy || (documentState.dirty && !window.confirm('Discard unsaved document changes?'))) return;
    documentState.dirty = false; $('#document-dialog').close();
  }
  $('#document-close').onclick = closeDocument;
  $('#document-dialog').addEventListener('cancel', event => {event.preventDefault(); closeDocument();});
  $('#document-content').addEventListener('input', () => {documentState.dirty = $('#document-content').value !== documentState.doc?.content; updateDocument();});
  $('#document-write').onclick = () => documentView(false);
  $('#document-preview').onclick = () => documentAction('preview');
  $('#document-save').onclick = () => documentAction('save');
  $('#document-copy').onclick = () => documentAction('copy');
  async function documentAction(action) {
    const d = documentState;
    if (d.busy || !d.doc || d.conflict) return;
    if (action === 'save' && d.doc.shared && !window.confirm(`Save changes to the shared document used by ${d.doc.usages.length} known references?`)) return;
    d.busy = true; updateDocument(); $('#document-error').textContent = '';
    try {
      const result = await api('/api/document', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({action, name: d.name, field: d.field, revision: d.doc.revision, file_revision: d.doc.file_revision, content: $('#document-content').value, shared: action === 'save' && d.doc.shared})});
      if (action === 'preview') {$('#document-rendered').srcdoc = result.html; documentView(true);}
      else {
        d.doc = result; d.dirty = false; $('#document-content').value = result.content;
        documentView(false); await reload(); notice(action === 'copy' ? 'Document copied and this Space reference updated.' : 'Document saved.');
      }
    } catch(error) {
      $('#document-error').textContent = error.message;
      if (error.status === 409) await reviewDocumentConflict();
    } finally {d.busy = false; updateDocument();}
  }
  async function reviewDocumentConflict() {
    const d = documentState;
    try {
      const latest = await api(`/api/document?name=${encodeURIComponent(d.name)}&field=${encodeURIComponent(d.field)}`);
      d.conflict = latest; documentView(false);
      const panel = $('#document-conflict'); panel.hidden = false;
      panel.replaceChildren(el('h3', {}, 'Saved Document Changed'), el('p', {}, `${d.doc.reference} -> ${latest.reference}`),
        el('div', {class: 'document-compare'}, el('label', {}, 'Previously loaded', el('textarea', { class: 'textarea',readOnly: true, value: d.doc.content})), el('label', {}, 'Currently saved', el('textarea', { class: 'textarea',readOnly: true, value: latest.content}))),
        el('div', {class: 'conflict-actions'}, el('button', {type: 'button', class: 'btn btn-sm btn-ghost', onclick: () => applyDocumentConflict(false)}, 'Use Saved Document'), el('button', {type: 'button', class: 'btn btn-sm btn-ghost', onclick: () => applyDocumentConflict(true)}, 'Keep My Draft')));
    } catch(error) {$('#document-error').textContent += ' ' + error.message + ' Your draft is preserved in the editor.';}
  }
  function applyDocumentConflict(keep) {
    const d = documentState;
    if (d.busy || !d.conflict) return;
    if (!keep && !window.confirm('Replace your document draft with the currently saved content?')) return;
    if (!keep) $('#document-content').value = d.conflict.content;
    d.doc = d.conflict; d.conflict = null; d.dirty = $('#document-content').value !== d.doc.content;
    $('#document-conflict').hidden = true; $('#document-error').textContent = ''; updateDocument();
  }

  const showValue = value => value === undefined ? '(not present)' : value === null ? '(removed)' : String(value);
  async function openRecovery(before) {
    if (state.busy || state.selected !== before.name) {notice('Return to the affected Space to review its draft.', true); return;}
    let draft; try {draft = mutation();} catch(error) {notice(error.message, true); return;}
    state.busy = true; updateSaveState();
    try {
      const snapshot = await api(), latest = snapshot.spaces.find(s => s.name === before.name && !s.trashed);
      if (!latest || !snapshot.write) throw new Error('This Space is no longer editable. Your draft is still in the form.');
      const rows = planRecovery(before, draft, latest, [...state.files.keys()]);
      for (const row of rows) if (row.pending && row.kind === 'meta' && (!snapshot.sharing_image || !snapshot.public_origin)) {row.available = false; row.choice = 'saved';}
      recovery = {snapshot, latest, rows, files: new Map(state.files)};
      $('#conflict-summary').textContent = 'Choose values for overlapping changes. Applying updates your draft; nothing is saved yet.';
      $('.form-error', $('#conflict-form')).textContent = '';
      $('#conflict-rows').replaceChildren(...rows.map(row => {
        const choice = el('select', { class: 'select','aria-label': 'Keep value for ' + row.label, required: true, disabled: !row.available || !row.changed, onchange: () => {row.choice = choice.value;}},
          el('option', {value: '', selected: !row.choice}, 'Choose a value'), el('option', {value: 'saved', selected: row.choice === 'saved'}, 'Currently saved'), el('option', {value: 'mine', selected: row.choice === 'mine'}, 'My draft'));
        return el('section', {class: 'conflict-row'}, el('div', {class: 'section-heading'}, el('h3', {}, row.label), choice),
          !row.available ? el('p', {class: 'blocked'}, 'This field or upload permission is no longer available. Its draft cannot be reapplied.') : null,
          row.pending ? el('p', {class: 'subtle'}, 'A selected file is pending upload.') : null,
          el('div', {class: 'value-comparison'}, ...[['Originally loaded', row.original], ['Currently saved', row.saved], ['My draft', row.mine]].map(([label, value]) => el('div', {}, el('strong', {}, label), el('pre', {}, showValue(value))))));
      }));
      if (!rows.length) $('#conflict-rows').append(el('p', {}, 'Only other source files changed. Your draft can be reloaded.'));
      $('#conflict-dialog').showModal(); icons();
    } catch(error) {notice(error.message, true);}
    finally {state.busy = false; updateSaveState();}
  }
  $('#conflict-form').addEventListener('submit', event => {
    event.preventDefault(); if (!recovery || state.busy) return;
    try {
      const {snapshot, latest, rows, files} = recovery, draft = resolveRecovery(rows, latest);
      state.snapshot = snapshot; initDraft(); state.meta = draft.meta; state.resets = new Set(draft.resets);
      state.files = new Map(draft.uploads.filter(id => files.has(id)).map(id => [id, files.get(id)]));
      render(draft.values);
      for (const key of ['title', 'route']) if (Object.hasOwn(draft, key)) $('#edit-form').elements[key].value = draft[key];
      dirty(); $('#conflict-dialog').close(); notice('Draft updated against current source. Review it, then Save Changes.');
      recovery = null;
    } catch(error) {$('.form-error', $('#conflict-form')).textContent = error.message;}
  });
  function openCreate() {
    if (!mayLeave()) return;
    closeDrawer();
    const form = $('#create-form'); form.reset(); $('.form-error', form).textContent = '';
    form.elements.source.replaceChildren(...state.snapshot.sources.map(s => el('option', { value: s.name }, s.name)));
    if (selected()?.source) form.elements.source.value = selected().source;
    $('#create-dialog').showModal();
  }
  $('#create-form').addEventListener('submit', async event => {
    event.preventDefault(); if (state.busy) return; const form = event.currentTarget;
    const m = { action: 'create', revision: state.snapshot.revision }; for (const k of ['source','name','title','route']) m[k] = form.elements[k].value;
    state.busy = true; $('button[type=submit]', form).disabled = true;
    try { await post(m); $('#create-dialog').close(); state.selected = m.name; state.trash = false; state.tab = 'content'; syncSelectionURL(m.name); state.busy = false; await reload(); notice('Space created.'); }
    catch (error) { $('.form-error', form).textContent = error.message; }
    finally { state.busy = false; $('button[type=submit]', form).disabled = false; updateSaveState(); }
  });
  async function trashSpace() {
    if (!mayLeave()) return; const s = selected();
    if (!window.confirm(`Move "${s.title}" to Trash? Route ${pageURL(s.route)} will be disabled.`)) return;
    state.busy = true; updateSaveState();
    try { await post({ action: 'trash', revision: state.snapshot.revision, name: s.name }); syncSelectionURL(null); state.busy = false; await reload(false); notice('Space moved to Trash.'); }
    catch (error) { notice(error.message, true); } finally { state.busy = false; updateSaveState(); }
  }
  async function restoreSpace() {
    if (state.busy) return; const s = selected(); state.busy = true;
    try { await post({ action: 'restore', revision: state.snapshot.revision, name: s.name }); state.trash = false; syncSelectionURL(s.name); state.busy = false; await reload(); notice('Space restored.'); }
    catch (error) { notice(error.message, true); } finally { state.busy = false; updateSaveState(); }
  }
  $('#meta-form').addEventListener('submit', event => {
    event.preventDefault(); const form = event.currentTarget; const key = form.elements.key.value.trim();
    if (!syncMeta()) return;
    if (Object.hasOwn(state.meta, key) && state.meta[key] !== null) { $('.form-error', form).textContent = 'This metadata key already exists.'; return; }
    state.meta[key] = form.elements.value.value; state.resets.delete(key); dirty(); renderMeta(); $('#meta-dialog').close();
  });
  $('#preset').addEventListener('change', event => { const form = $('#meta-form'); form.elements.key.value = event.target.value; form.elements.value.value = event.target.value === 'og:type' ? 'website' : ''; });
  document.querySelectorAll('dialog .close').forEach(button => button.addEventListener('click', () => { if (!state.busy) button.closest('dialog').close(); }));
  document.querySelectorAll('dialog').forEach(dialog => dialog.addEventListener('cancel', event => { if (state.busy) event.preventDefault(); }));
  $('#new').addEventListener('click', openCreate);
  $('#search').addEventListener('input', () => { if (state.snapshot) renderList(); });
  $('#active-tab').addEventListener('click', () => { if (!state.trash || !mayLeave()) return; state.trash = false; state.selected = null; syncSelectionURL(null); notice(); reload().catch(e => notice(e.message, true)); });
  $('#trash-tab').addEventListener('click', () => { if (state.trash || !mayLeave()) return; state.trash = true; state.selected = null; syncSelectionURL(null); notice(); reload().catch(e => notice(e.message, true)); });
  $('#reload').addEventListener('click', () => { if (mayLeave()) { notice(); reload().catch(e => notice(e.message, true)); } });
  window.addEventListener('popstate', () => { if (state.snapshot) navigateFromLocation(); });
  window.addEventListener('beforeunload', event => { if (state.dirty || state.busy || documentState.dirty || documentState.busy) { event.preventDefault(); event.returnValue = ''; } });
  icons(); reload().catch(error => { notice(error.message, true); $('#main').replaceChildren(el('div', { class: 'empty' }, icon('triangle-alert'), el('h2', {}, 'Could not load Spaces'))); icons(); });
})();
