import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';

// Exercise the adapter's event handlers without loading HTMX or adding a DOM dependency.
const source = readFileSync(new URL('../resources/js/catalog.js', import.meta.url), 'utf8')
  .replace(/^import "\.\.\/vendor\/htmx-4\.0\.0\.js";\n/, '');

function fixture(panelID = 'catalog-panel') {
  const handlers = new Map();
  const elements = new Map();
  const document = {
    body: {id: '', isConnected: true},
    getElementById: id => elements.get(id) ?? null,
    addEventListener: (name, handler) => handlers.set(name, [...handlers.get(name) ?? [], handler]),
  };
  document.activeElement = document.body;
  const element = (id, parent = null, options = {}) => {
    const value = {
      id, parent, disabled: false, hidden: false, isConnected: true,
      contains: child => child === value || child?.parent === value,
      focus: () => { document.activeElement = value; },
      ...options,
    };
    elements.set(id, value);
    return value;
  };
  const oldPanel = element(panelID);
  const search = element('search');
  const emit = (name, ctx) => handlers.get(name)?.forEach(handler => handler({detail: {ctx}}));
  const ctx = {target: oldPanel};
  runInNewContext(source, {document});
  return {
    document, element, search, oldPanel,
    before: () => emit('htmx:before:swap', ctx),
    replace: () => {
      for (const [id, value] of elements) if (oldPanel.contains(value)) {
        value.isConnected = false;
        elements.delete(id);
      }
      if (oldPanel.contains(document.activeElement)) document.activeElement = document.body;
      return element(panelID);
    },
    after: () => emit('htmx:after:swap', ctx),
  };
}

test('replaced enabled pagination button retains keyboard focus', () => {
  const f = fixture();
  f.element('next-page', f.oldPanel).focus();
  f.before();
  const next = f.element('next-page', f.replace());
  f.after();
  assert.equal(f.document.activeElement, next);
});

test('a newly disabled pagination button falls back to page status', () => {
  const f = fixture();
  f.element('next-page', f.oldPanel).focus();
  f.before();
  const panel = f.replace();
  f.element('next-page', panel, {disabled: true});
  const status = f.element('page-status', panel);
  f.after();
  assert.equal(f.document.activeElement, status);
});

test('after:swap repairs a disabled replacement still reported as active by the browser', () => {
  const f = fixture();
  f.element('next-page', f.oldPanel).focus();
  f.before();
  const panel = f.replace();
  const next = f.element('next-page', panel, {disabled: true});
  f.document.activeElement = next;
  const status = f.element('page-status', panel);
  f.after();
  assert.equal(f.document.activeElement, status);
});

test('after:swap repairs the removed source still temporarily reported as active', () => {
  const f = fixture();
  const retry = f.element('retry-load', f.oldPanel);
  retry.focus();
  f.before();
  const status = f.element('page-status', f.replace());
  f.document.activeElement = retry;
  assert.equal(retry.isConnected, false);
  f.after();
  assert.equal(f.document.activeElement, status);
});

test('successful retry focuses page status after the retry button disappears', () => {
  const f = fixture();
  f.element('retry-load', f.oldPanel).focus();
  f.before();
  const status = f.element('page-status', f.replace());
  f.after();
  assert.equal(f.document.activeElement, status);
});

test('failed pagination moves focus to retry, including a source removed by the swap', () => {
  const f = fixture();
  f.element('previous-page', f.oldPanel).focus();
  f.before();
  const retry = f.element('retry-load', f.replace());
  f.after();
  assert.equal(f.document.activeElement, retry);
});

test('input focus is preserved during live search and category updates', () => {
  const f = fixture();
  f.search.focus();
  f.before();
  f.element('page-status', f.replace());
  f.after();
  assert.equal(f.document.activeElement, f.search);
});

test('focus moved outside the panel during a swap is not stolen', () => {
  const f = fixture();
  f.element('next-page', f.oldPanel).focus();
  f.before();
  f.element('page-status', f.replace());
  f.search.focus();
  f.after();
  assert.equal(f.document.activeElement, f.search);
});

test('focus already restored by HTMX is left untouched', () => {
  const f = fixture();
  f.element('next-page', f.oldPanel).focus();
  f.before();
  const panel = f.replace();
  const next = f.element('next-page', panel);
  next.focus();
  f.element('page-status', panel);
  f.after();
  assert.equal(f.document.activeElement, next);
});

test('hidden replacements are skipped and result status remains a fallback', () => {
  const f = fixture();
  f.element('clear-filters', f.oldPanel).focus();
  f.before();
  const panel = f.replace();
  f.element('clear-filters', panel, {hidden: true});
  const status = f.element('result-count', panel);
  f.after();
  assert.equal(f.document.activeElement, status);
});

test('cart quantity controls retain their identity after the panel is replaced', () => {
  const f = fixture('cart-panel');
  f.element('cart-increase-7', f.oldPanel).focus();
  f.before();
  const increase = f.element('cart-increase-7', f.replace());
  f.after();
  assert.equal(f.document.activeElement, increase);
});

test('disabled cart quantity control focuses the updated summary', () => {
  const f = fixture('cart-panel');
  f.element('cart-increase-7', f.oldPanel).focus();
  f.before();
  const panel = f.replace();
  f.document.activeElement = f.element('cart-increase-7', panel, {disabled: true});
  const summary = f.element('cart-summary', panel);
  f.after();
  assert.equal(f.document.activeElement, summary);
});

for (const remaining of [true, false]) {
  test(`removing a cart item focuses ${remaining ? 'summary' : 'empty state'}`, () => {
    const f = fixture('cart-panel');
    const remove = f.element('cart-remove-7', f.oldPanel);
    remove.focus();
    f.before();
    const destination = f.element(remaining ? 'cart-summary' : 'cart-empty', f.replace());
    // Removed active controls can still be reported during HTMX's after:swap.
    f.document.activeElement = remove;
    f.after();
    assert.equal(f.document.activeElement, destination);
  });
}

test('successful checkout focuses the order confirmation heading', () => {
  const f = fixture('checkout-content');
  const submit = f.element('checkout-submit', f.oldPanel);
  submit.focus();
  f.before();
  const confirmation = f.element('checkout-confirmation', f.replace());
  f.document.activeElement = submit;
  f.after();
  assert.equal(f.document.activeElement, confirmation);
});

test('checkout with an empty cart focuses its explanatory heading', () => {
  const f = fixture('checkout-content');
  f.element('checkout-submit', f.oldPanel).focus();
  f.before();
  const empty = f.element('checkout-empty', f.replace());
  f.after();
  assert.equal(f.document.activeElement, empty);
});

test('checkout response keeps focus on a surviving field', () => {
  const f = fixture('checkout-content');
  f.element('checkout-email', f.oldPanel).focus();
  f.before();
  const email = f.element('checkout-email', f.replace());
  f.after();
  assert.equal(f.document.activeElement, email);
});

test('swaps outside the three owned panels do not move focus', () => {
  const f = fixture('unrelated-panel');
  f.element('other-control', f.oldPanel).focus();
  f.before();
  f.element('other-control', f.replace());
  f.after();
  assert.equal(f.document.activeElement, f.document.body);
});
