import test from 'node:test';
import assert from 'node:assert/strict';
import {fieldGroups, planRecovery, resolveRecovery} from './web/recovery.mjs';

const field = (id, value, extra = {}) => ({id, value, label: id, type: 'text', ...extra});
const original = () => ({title: 'Original', route: 'page', fields: [field('name', 'Name'), field('intro', 'Welcome')], meta: {author: 'A', description: 'Old'}, source_meta: {author: 'A', description: 'Old'}});
const mutation = () => ({title: 'Original', route: 'page', values: {name: 'Name', intro: 'Welcome'}, meta: {}, reset_meta: []});

test('nonoverlapping edits keep saved changes and reapply only the draft', () => {
  const before = original(), latest = original(), draft = mutation();
  latest.title = 'External'; draft.values.intro = 'My introduction';
  const rows = planRecovery(before, draft, latest);
  assert.equal(rows.some(r => r.conflict), false);
  assert.deepEqual(resolveRecovery(rows, latest), {values: {intro: 'My introduction'}, meta: latest.meta, resets: [], uploads: []});
});
test('overlap requires a choice; applying does not mutate any input', () => {
  const before = original(), latest = original(), draft = mutation();
  latest.title = 'External'; draft.title = 'Mine';
  const rows = planRecovery(before, draft, latest);
  assert.throws(() => resolveRecovery(rows, latest), /Choose/);
  rows[0].choice = 'mine';
  assert.equal(resolveRecovery(rows, latest).title, 'Mine');
  assert.equal(latest.title, 'External'); assert.equal(before.title, 'Original');
});
test('removed fields and removed upload permission cannot be restored by a draft', () => {
  const before = original(), latest = original(), draft = mutation();
  draft.values.intro = 'Keep locally'; latest.fields.pop();
  const rows = planRecovery(before, draft, latest);
  rows.find(r => r.key === 'intro').choice = 'mine';
  assert.deepEqual(resolveRecovery(rows, latest).values, {});
  before.fields[0].upload = {accept: ['.png']};
  const pending = planRecovery(before, mutation(), latest, ['name']);
  assert.equal(pending.find(r => r.key === 'name').available, false);
});
test('changed field schema requires review even when the value is unchanged on disk', () => {
  const before = original(), latest = original(), draft = mutation();
  draft.values.name = 'New name'; latest.fields[0].max = 4;
  assert.equal(planRecovery(before, draft, latest).find(r => r.key === 'name').choice, '');
});
test('metadata reset uses the latest source and remains distinct from removal', () => {
  const before = original(), latest = original(), draft = mutation();
  draft.meta.author = null; draft.reset_meta = ['description'];
  latest.source_meta.description = 'Updated source';
  const rows = planRecovery(before, draft, latest);
  const result = resolveRecovery(rows, latest);
  assert.equal(result.meta.author, null); assert.equal(result.meta.description, 'Updated source');
  assert.deepEqual(result.resets, ['description']);
});
test('pending uploads are retained only when the user keeps the draft', () => {
  const before = original(), latest = original(), draft = mutation();
  before.fields[0].upload = latest.fields[0].upload = {accept: ['.png']};
  const rows = planRecovery(before, draft, latest, ['name']);
  assert.deepEqual(resolveRecovery(rows, latest).uploads, ['name']);
  rows[0].choice = 'saved'; assert.deepEqual(resolveRecovery(rows, latest).uploads, []);
});
test('field groups follow first occurrence and retain supplied order', () => {
  const groups = fieldGroups([field('name', '', {group: 'Intro'}), field('image', '', {group: 'Assets'}), field('welcome', '', {group: 'Intro'}), field('email', '')]);
  assert.deepEqual(groups.map(g => g.name), ['Intro', 'Assets', '']);
  assert.deepEqual(groups[0].fields.map(f => f.id), ['name', 'welcome']);
});
