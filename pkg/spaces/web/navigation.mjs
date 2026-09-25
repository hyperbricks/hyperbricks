// Field identities are the catalog's canonical JSON Pointers. Do not translate
// dotted paths: a field name can itself contain a dot, slash, or tilde.
export function readNavigation(search) {
  const params = new URLSearchParams(search);
  if (!params.has('name') && !params.has('field')) return null;
  if (params.getAll('name').length !== 1 || params.getAll('field').length > 1) {
    return {error: 'This edit link must identify exactly one Space and at most one field.'};
  }
  const name = params.get('name');
  if (!name) return {error: 'This edit link is missing its Space name.'};
  const field = params.get('field');
  if (field !== null && (!field.startsWith('/') || /~(?![01])/u.test(field))) {
    return {name, error: 'This edit link has an invalid field ID. Use the field’s canonical path, such as /content/values/title.'};
  }
  return {name, field};
}

export function resolveNavigation(spaces, target) {
  if (!target) return null;
  if (target.error && !target.name) return {space: null, field: null, error: target.error};
  const space = spaces.find(candidate => candidate.name === target.name);
  if (!space) return {space: null, field: null, error: `Space “${target.name}” was not found. It may have been renamed or removed. Choose an available Space from the list.`};
  if (space.trashed) return {space: null, field: null, error: `Space “${target.name}” is in Trash. Restore it from Trash before editing this field.`};
  if (target.error) return {space, field: null, error: target.error};
  if (target.field === null) return {space, field: null, error: ''};
  const field = space.fields.find(candidate => candidate.id === target.field);
  if (!field) return {space, field: null, error: `Field “${target.field}” is not editable in Space “${space.name}”. It may have been renamed or removed from the source’s editable declaration.`};
  return {space, field, error: ''};
}

export function selectionURL(currentURL, name, field = null) {
  const url = new URL(currentURL);
  url.searchParams.delete('name');
  url.searchParams.delete('field');
  if (name) {
    url.searchParams.set('name', name);
    if (field !== null) url.searchParams.set('field', field);
  }
  return url.href;
}

export function editPageURL(route, currentURL) {
  const path = '/' + (route === 'index' ? '' : route.replace(/^\/+/, ''));
  const url = new URL(path, currentURL);
  url.searchParams.set('edit', 'true');
  return url.pathname + url.search + url.hash;
}
