export function fieldGroups(fields) {
  const groups = new Map();
  for (const field of fields) {
    const key = field.group || '';
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(field);
  }
  return [...groups].map(([name, fields]) => ({name, fields}));
}

const own = (object, key) => Object.hasOwn(object || {}, key);
const schema = field => JSON.stringify(field && [field.type, field.required, field.max, field.upload, field.directory]);

// Build a three-way comparison without mutating the current form or saved state.
export function planRecovery(before, draft, latest, pending = []) {
  const rows = [];
  const add = (kind, key, label, original, mine, saved, available = true, extra = {}) => {
    const changed = original !== mine || extra.pending || extra.reset;
    if (!changed && original === saved) return;
    const conflict = changed && (extra.schemaChanged || (saved !== original && saved !== mine));
    rows.push({kind, key, label, original, mine, saved, available, changed, conflict,
      choice: !available || !changed ? 'saved' : conflict ? '' : 'mine', ...extra});
  };
  for (const key of ['title', 'route']) add('identity', key, key === 'title' ? 'Title' : 'Route', before[key], draft[key], latest[key]);
  const oldFields = new Map(before.fields.map(f => [f.id, f]));
  const newFields = new Map(latest.fields.map(f => [f.id, f]));
  for (const key of new Set([...oldFields.keys(), ...newFields.keys(), ...Object.keys(draft.values)])) {
    const old = oldFields.get(key), next = newFields.get(key), upload = pending.includes(key);
    add('field', key, next?.label || old?.label || key, old?.value,
      own(draft.values, key) ? draft.values[key] : old?.value, next?.value,
      !!next && (!upload || !!next.upload), {pending: upload, schemaChanged: !!old && !!next && schema(old) !== schema(next)});
  }
  const resets = new Set(draft.reset_meta);
  for (const key of new Set([...Object.keys(before.meta), ...Object.keys(latest.meta), ...Object.keys(draft.meta), ...resets])) {
    const reset = resets.has(key);
    const mine = reset ? latest.source_meta[key] : own(draft.meta, key) ? draft.meta[key] : before.meta[key];
    const upload = key === 'og:image' && pending.includes('@meta.og:image');
    add('meta', key, key, before.meta[key], mine, latest.meta[key], true, {reset, pending: upload});
  }
  return rows;
}

export function resolveRecovery(rows, latest) {
  const result = {values: {}, meta: {...latest.meta}, resets: [], uploads: []};
  for (const row of rows) {
    if (!row.choice) throw new Error(`Choose which value to keep for ${row.label}.`);
    if (!row.available || row.choice !== 'mine') continue;
    if (row.kind === 'identity') result[row.key] = row.mine;
    if (row.kind === 'field') result.values[row.key] = row.mine;
    if (row.kind === 'meta') {
      if (row.mine === undefined) delete result.meta[row.key]; else result.meta[row.key] = row.mine;
      if (row.reset) result.resets.push(row.key);
    }
    if (row.pending) result.uploads.push(row.kind === 'meta' ? '@meta.og:image' : row.key);
  }
  return result;
}
