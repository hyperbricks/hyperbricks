import test from 'node:test';
import assert from 'node:assert/strict';
import {readNavigation, resolveNavigation, selectionURL, editPageURL} from './web/navigation.mjs';

const skip = {id: '/content/values/skip_link', label: 'Skip-link text'};
const punctuation = {id: '/content/values/a.b~1c~0d', label: 'Punctuation'};
const spaces = [
  {name: 'landing_de', trashed: false, fields: [skip]},
  {name: 'landing_en', trashed: false, fields: [skip, punctuation]},
  {name: 'old_page', trashed: true, fields: [skip]},
];

test('a deep link selects the named Space and exact decoded catalog field', () => {
  const result = resolveNavigation(spaces, readNavigation('?name=landing_en&field=%2Fcontent%2Fvalues%2Fskip_link'));
  assert.equal(result.space, spaces[1]);
  assert.equal(result.field, skip);
  assert.equal(result.error, '');
});

test('Space-only links work and unrelated parameters do not create a target', () => {
  assert.equal(readNavigation('?filter=all'), null);
  assert.equal(resolveNavigation(spaces, null), null);
  assert.deepEqual(readNavigation('?name=landing_en'), {name: 'landing_en', field: null});
  assert.equal(resolveNavigation(spaces, readNavigation('?name=landing_en')).field, null);
});

test('unknown and trashed Space names never fall back to the first Space', () => {
  for (const [name, message] of [['missing', /not found/], ['old_page', /in Trash/]]) {
    const result = resolveNavigation(spaces, readNavigation(`?name=${name}&field=${encodeURIComponent(skip.id)}`));
    assert.equal(result.space, null);
    assert.equal(result.field, null);
    assert.match(result.error, message);
  }
});

test('stale fields leave the requested Space open with a specific error', () => {
  const result = resolveNavigation(spaces, readNavigation('?name=landing_en&field=/content/values/removed'));
  assert.equal(result.space, spaces[1]);
  assert.equal(result.field, null);
  assert.match(result.error, /not editable.*landing_en/);
});

test('field identities keep JSON Pointer escaping and literal dots', () => {
  const target = readNavigation(`?name=landing_en&field=${encodeURIComponent(punctuation.id)}`);
  assert.equal(target.field, punctuation.id);
  assert.equal(resolveNavigation(spaces, target).field, punctuation);
  assert.match(readNavigation('?name=landing_en&field=content.values.skip_link').error, /invalid field ID/);
  assert.match(readNavigation('?name=landing_en&field=/content/values/a~2b').error, /invalid field ID/);
});

test('ambiguous and incomplete links produce errors rather than a default selection', () => {
  for (const search of ['?name=', '?field=/x', '?name=landing_en&name=landing_de', '?name=landing_en&field=/x&field=/y']) {
    const target = readNavigation(search);
    assert.ok(target.error, search);
    assert.equal(resolveNavigation(spaces, target).space, null, search);
  }
});

test('selection URLs round-trip field identity and preserve unrelated query and hash', () => {
  const href = selectionURL('http://localhost:8092/custom/spaces?view=compact&name=old&field=/old#editor', 'landing_en', punctuation.id);
  const url = new URL(href);
  assert.equal(url.pathname, '/custom/spaces');
  assert.equal(url.searchParams.get('view'), 'compact');
  assert.equal(url.hash, '#editor');
  assert.deepEqual(readNavigation(url.search), {name: 'landing_en', field: punctuation.id});
  const cleared = new URL(selectionURL(href, null));
  assert.equal(cleared.search, '?view=compact');
});

test('edit-page URLs handle index, existing queries, fragments, and repeated edit parameters', () => {
  const current = 'http://localhost:8092/__hyperbricks/spaces?name=landing_en';
  assert.equal(editPageURL('index', current), '/?edit=true');
  assert.equal(editPageURL('/docs?category=api&edit=false&edit=false#examples', current), '/docs?category=api&edit=true#examples');
  assert.equal(editPageURL('de', current), '/de?edit=true');
  assert.equal(editPageURL('//external.example', current), '/external.example?edit=true');
});
