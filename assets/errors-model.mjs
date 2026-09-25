export const diagnosticEndpoint = '/__hyperbricks/render-diagnostics';

export function stripANSI(value) {
  return String(value ?? '').replace(/\x1b\[[0-?]*[ -/]*[@-~]/g, '');
}

export function severity(issue) {
  if (!issue.rejected && String(issue.level).toLowerCase() === 'info') return 'info';
  return !issue.rejected && ['warning', 'warn'].includes(String(issue.level).toLowerCase()) ? 'warning' : 'error';
}

export function recordSeverity(record) {
  return record.errors.some(issue => severity(issue) === 'error') ? 'error' : record.errors.some(issue => severity(issue) === 'warning') ? 'warning' : 'info';
}

export function validateCurrentDiagnostics(value) {
  const count = value => Number.isInteger(value) && value >= 0;
  if (!value || !count(value.generation) || !count(value.checked_routes) || !count(value.total_routes) ||
      value.checked_routes > value.total_routes || !count(value.evicted_contexts) ||
      !Array.isArray(value.unchecked_routes) || value.unchecked_routes.some(route => typeof route !== 'string') ||
      value.unchecked_routes.length !== value.total_routes - value.checked_routes) {
    throw new Error('The runtime returned an invalid current diagnostics response.');
  }
  validateDiagnostics(value.records);
  if (value.records.some(record => record.generation !== value.generation || typeof record.context_id !== 'string' || !record.context_id)) {
    throw new Error('The runtime returned an invalid current diagnostics response.');
  }
  return value;
}

export function sourceLocation(file, line, column) {
  if (!file || file === 'Unknown') return 'Not provided';
  return file + (line > 0 ? `:${line}${column > 0 ? ':' + column : ''}` : '');
}

export function currentSelection(records, requestID, contextID) {
  return records.find(record => record.request_id === requestID) ||
    (contextID ? records.find(record => record.context_id === contextID) : undefined);
}

export function routeLabel(route) {
  if (route === '__config') return 'Configuration';
  return route === 'index' ? '/' : '/' + route.replace(/^\/+/, '');
}

export function validateDiagnostics(value) {
  if (!Array.isArray(value) || value.some(record => !record ||
      typeof record.request_id !== 'string' || !record.request_id ||
      typeof record.route !== 'string' || typeof record.created_at !== 'string' ||
      !Number.isFinite(Date.parse(record.created_at)) || !Array.isArray(record.errors) ||
      !record.errors.length || record.errors.some(issue => !issue || typeof issue.err !== 'string'))) {
    throw new Error('The runtime returned an invalid diagnostics response.');
  }
  return value;
}

export function filterRecords(records, query, level) {
  const needle = query.trim().toLowerCase();
  return records.filter(record => (level === 'all' || record.errors.some(issue => severity(issue) === level)) &&
    [record.request_id, record.method, routeLabel(record.route), ...record.errors.flatMap(issue =>
      [issue.err, issue.file, issue.type, issue.path, issue.key, issue.resource, issue.phase])].join(' ').toLowerCase().includes(needle));
}

export function diagnosticURL(requestID) {
  return `${diagnosticEndpoint}?${new URLSearchParams({request_id: requestID})}`;
}

export async function readJSON(url, fetcher = fetch) {
  let response;
  try { response = await fetcher(url, {cache: 'no-store', headers: {Accept: 'application/json'}}); }
  catch { throw new Error('Cannot reach the runtime. Check that it is running, then refresh.'); }
  if (!response.ok) {
    if (response.status === 404) throw new Error('Diagnostics are unavailable. Check that the runtime is running in development or debug mode with the Dashboard enabled.');
    if (response.status === 429) throw new Error('Too many requests. Wait a moment, then refresh.');
    throw new Error(`The runtime could not return diagnostics (HTTP ${response.status}). Try refreshing once the runtime is available.`);
  }
  try { return await response.json(); }
  catch { throw new Error('The runtime returned an unreadable response. Refresh after checking the server.'); }
}
