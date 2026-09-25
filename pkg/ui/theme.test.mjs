import {readFileSync} from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';

const source = readFileSync(new URL('./web/theme.js', import.meta.url), 'utf8');
function setup({stored = null, dark = false, blocked = false, labels = {}} = {}) {
  const events = {}, buttonEvents = {}, attrs = {};
  const root = {dataset: {}};
  const button = {dataset: labels, setAttribute: (name,value) => {attrs[name] = value;}, addEventListener: (name,fn) => {buttonEvents[name] = fn;}};
  const media = {matches: dark, addEventListener: (name,fn) => {events.media = fn;}};
  const storage = {getItem: () => {if (blocked) throw Error('blocked'); return stored;}, setItem: (key,value) => {if (blocked) throw Error('blocked'); stored = value;}};
  vm.runInNewContext(source, {
    window: {matchMedia: () => media, addEventListener: (name,fn) => {events[name] = fn;}},
    document: {documentElement: root, querySelectorAll: () => [button], addEventListener: (name,fn) => {events[name] = fn;}},
    localStorage: storage
  });
  events.DOMContentLoaded();
  return {root, attrs, events, media, click: () => buttonEvents.click(), stored: () => stored, store: value => {stored = value;}};
}
test('theme is applied before paint, and legacy choices are preserved', () => {
  for (const [stored,expected] of [['light','lofi'],['dark','night'],['lofi','lofi'],['night','night']]) {
    const ui = setup({stored});
    assert.equal(ui.root.dataset.theme, expected);
    assert.match(ui.attrs['aria-label'], expected === 'night' ? /light/ : /dark/);
  }
});
test('invalid preferences follow the system until the user chooses a theme', () => {
  const ui = setup({stored:'unknown', dark:true});
  assert.equal(ui.root.dataset.theme,'night');
  ui.media.matches = false; ui.events.media();
  assert.equal(ui.root.dataset.theme,'lofi');
  ui.click();
  assert.equal(ui.stored(),'night');
  ui.events.media();
  assert.equal(ui.root.dataset.theme,'night');
});
test('theme choice still works with unavailable storage', () => {
  const ui = setup({blocked:true});
  ui.click();
  assert.equal(ui.root.dataset.theme,'night');
  assert.equal(ui.attrs['aria-pressed'],'true');
});
test('other tabs and cleared storage synchronize without reloading', () => {
  const ui = setup({stored:'night'});
  ui.store('lofi'); ui.events.storage({key:'hb-theme'});
  assert.equal(ui.root.dataset.theme,'lofi');
  ui.store(null); ui.media.matches=true; ui.events.storage({key:null});
  assert.equal(ui.root.dataset.theme,'night');
});
test('localized labels follow the selected theme and retain English defaults', () => {
  const ui = setup({labels:{themeLightLabel:'Licht thema',themeDarkLabel:'Donker thema'}});
  assert.equal(ui.attrs['aria-label'],'Donker thema');
  assert.equal(ui.attrs.title,'Donker thema');
  ui.click();
  assert.equal(ui.attrs['aria-label'],'Licht thema');
  assert.equal(ui.attrs.title,'Licht thema');
  const partial = setup({stored:'night',labels:{themeDarkLabel:'Donker thema'}});
  assert.equal(partial.attrs['aria-label'],'Switch to light theme');
});
