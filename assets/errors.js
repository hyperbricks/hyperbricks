import {diagnosticEndpoint, stripANSI, severity, recordSeverity, routeLabel,
  validateCurrentDiagnostics, sourceLocation, currentSelection, filterRecords, diagnosticURL, readJSON} from './errors-model.mjs';

const $ = id => document.getElementById(id);
let records = [], loaded = false, selectedID = new URL(location.href).searchParams.get('request_id');
let selectedContext, snapshot, previousSnapshot = '';
let copyTimer;
const text = (tag, value, className = '') => {
  const element = document.createElement(tag);
  element.textContent = stripANSI(value);
  element.className = className;
  return element;
};
const timestamp = value => new Date(value).toLocaleString(undefined, {dateStyle:'medium', timeStyle:'medium'});
const levelLabel = level => ({warning:'Warning', error:'Error', info:'Information'}[level] || level);
const badge = level => text('span', levelLabel(level), `badge badge-sm badge-${['warning','error','info'].includes(level) ? level : 'ghost'}`);

function renderList() {
  const focusedContext = document.activeElement?.dataset.contextId;
  const visible = filterRecords(records, $('search').value, $('severity').value);
  $('requestCount').textContent = visible.length;
  $('requests').replaceChildren(...visible.map(record => {
    const button = text('button', '', 'errors-request');
    button.type = 'button';
    button.dataset.contextId = record.context_id;
    button.setAttribute('aria-current', String(record.request_id === selectedID));
    button.setAttribute('aria-controls', 'detail');
    const heading = text('span', '', 'errors-request-top');
    heading.append(text('strong', routeLabel(record.route)), badge(recordSeverity(record)));
    const meta = text('span', '', 'errors-request-meta hb-muted');
    const time = text('time', timestamp(record.created_at));
    time.dateTime = record.created_at;
    meta.append(text('code', record.request_id), time);
    button.append(heading, text('span', record.errors[0].err, 'errors-request-message'), meta);
    if (record.method) button.append(text('span', `${record.method} · Context ${record.context_id.slice(0,8)}`, 'hb-muted errors-request-meta'));
    button.addEventListener('click', () => {
      selectedID = record.request_id;
      selectedContext = record.context_id;
      const url = new URL(location.href);
      url.searchParams.set('request_id', selectedID);
      history.replaceState(null, '', url);
      // Update selection in place so keyboard focus remains on the chosen row.
      [...$('requests').children].forEach(row => row.setAttribute('aria-current', String(row === button)));
      renderDetail();
    });
    return button;
  }));
  if (focusedContext) [...$('requests').children].find(row => row.dataset.contextId === focusedContext)?.focus();
  $('listEmpty').hidden = visible.length > 0 || !loaded;
  const filtered = records.length > 0;
  $('emptyTitle').textContent = filtered ? 'No matching diagnostics' : 'No current diagnostics';
  $('emptyBody').textContent = filtered ? 'No current diagnostic matches these filters.' :
    !snapshot?.checked_routes ? 'No routes have been checked since the last configuration reload.' :
      'No diagnostics in the latest completed checks. Unchecked routes and other request variants may still contain errors.';
  $('clearFilters').hidden = !filtered;
}

function renderDetail() {
  const record = records.find(record => record.request_id === selectedID);
  $('selection').hidden = !record;
  $('selectionEmpty').hidden = Boolean(record);
  $('detail').setAttribute('aria-labelledby', record ? 'recordRoute' : 'detail-title');
  clearTimeout(copyTimer);
  $('copyStatus').textContent = '';
  if (!record) {
    $('selectionMessage').textContent = !loaded ? 'Current diagnostic details are unavailable.' : selectedID
      ? `Request ${selectedID} has no current diagnostic. Its result may have been resolved, replaced, or invalidated by a reload.`
      : loaded ? 'No diagnostic request is selected.' : 'Diagnostics have not loaded yet.';
    return;
  }
  $('recordRoute').textContent = routeLabel(record.route);
  const level = recordSeverity(record);
  $('recordSeverity').className = `badge badge-sm badge-${level}`;
  $('recordSeverity').textContent = levelLabel(level);
  $('requestID').textContent = record.request_id;
  $('recordTime').textContent = timestamp(record.created_at);
  $('recordTime').dateTime = record.created_at;
  $('rawLink').href = diagnosticURL(record.request_id);
  $('rawJSON').textContent = JSON.stringify(record, null, 2);
  $('issues').replaceChildren(...record.errors.map((issue, index) => {
    const row = text('li', '', 'errors-issue');
    row.dataset.severity = severity(issue);
    const heading = text('div', '', 'errors-issue-heading');
    heading.append(badge(severity(issue)), text('h3', issue.type && issue.type !== 'Unknown' ? issue.type : `Diagnostic ${index + 1}`));
    if (issue.rejected) heading.append(text('span', 'Rejected', 'badge badge-sm badge-outline'));
    row.append(heading, text('p', issue.err, 'errors-message'));
    const location = text('dl', '', 'errors-location');
    for (const [label, value, wide] of [
      ['Source', sourceLocation(issue.file, issue.line, issue.column), true],
      ['Component path', issue.path], ['Key', issue.key],
      ['Resource', sourceLocation(issue.resource, issue.resource_line, issue.resource_column), true],
      ['Phase', issue.phase],
      ['Execution position', issue.resource_line > 0 ? `Line ${issue.resource_line}${issue.resource_column > 0 ? ', column ' + issue.resource_column : ''}` : 'Not provided'],
    ]) {
      const field = text('div', '', wide ? 'errors-file' : '');
      field.append(text('dt', label), text('dd', value && value !== 'Unknown' ? value : 'Not provided'));
      location.append(field);
    }
    row.append(location);
    return row;
  }));
}

async function refresh() {
  if ($('refresh').disabled) return;
  $('refresh').disabled = true;
  $('requests').setAttribute('aria-busy', 'true');
  let changed = true;
  try {
    snapshot = validateCurrentDiagnostics(await readJSON(`${diagnosticEndpoint}?view=current`));
    const serialized = JSON.stringify(snapshot);
    changed = serialized !== previousSnapshot || !loaded;
    previousSnapshot = serialized;
    records = snapshot.records;
    loaded = true;
    if (!selectedID && records.length) selectedID = records[0].request_id;
    const selected = currentSelection(records, selectedID, selectedContext);
    if (selected) {
      selectedID = selected.request_id;
      selectedContext = selected.context_id;
      const url = new URL(location.href);
      url.searchParams.set('request_id', selectedID);
      history.replaceState(null, '', url);
    }
    $('loadError').hidden = true;
    $('updated').textContent = `Refreshed ${new Date().toLocaleTimeString()}`;
    $('coverage').textContent = `${snapshot.checked_routes} of ${snapshot.total_routes} routes checked since reload · Generation ${snapshot.generation}`;
    $('uncheckedPanel').hidden = !snapshot.unchecked_routes.length;
    $('uncheckedCount').textContent = snapshot.unchecked_routes.length;
    if (changed) $('uncheckedRoutes').replaceChildren(...snapshot.unchecked_routes.map(route => text('li', routeLabel(route))));
    $('limitedCoverage').hidden = !snapshot.evicted_contexts;
  } catch (error) {
    records = [];
    loaded = false;
    snapshot = undefined;
    $('loadError').hidden = false;
    const cause = error?.message || 'The runtime could not be reached.';
    $('loadError').textContent = `Current error status is unavailable. ${cause}`;
    $('coverage').textContent = 'Current render status could not be verified.';
    $('updated').textContent = '';
    $('uncheckedPanel').hidden = true;
    $('limitedCoverage').hidden = true;
  }
  const issues = records.flatMap(record => record.errors);
  const errors = issues.filter(issue => severity(issue) === 'error').length;
  const warnings = issues.filter(issue => severity(issue) === 'warning').length;
  const information = issues.length - errors - warnings;
  $('summary').textContent = loaded
    ? `${errors} current ${errors === 1 ? 'error' : 'errors'} · ${warnings} ${warnings === 1 ? 'warning' : 'warnings'}${information ? ' · ' + information + ' informational' : ''}`
    : 'Diagnostics unavailable';
  if (changed) { renderList(); renderDetail(); }
  $('requests').setAttribute('aria-busy', 'false');
  $('refresh').disabled = false;
}

$('refresh').addEventListener('click', refresh);
$('search').addEventListener('input', renderList);
$('severity').addEventListener('change', renderList);
$('clearFilters').addEventListener('click', () => { $('search').value = ''; $('severity').value = 'all'; renderList(); $('search').focus(); });
$('copy').addEventListener('click', async () => {
  const record = records.find(record => record.request_id === selectedID);
  if (!record) return;
  const requestID = selectedID;
  try {
    await navigator.clipboard.writeText(JSON.stringify(record, null, 2));
    if (selectedID === requestID) $('copyStatus').textContent = 'Diagnostic JSON copied.';
  } catch {
    if (selectedID === requestID) $('copyStatus').textContent = 'Copy unavailable. Open Raw JSON to copy the diagnostic.';
  }
  clearTimeout(copyTimer);
  copyTimer = setTimeout(() => { $('copyStatus').textContent = ''; }, 4000);
});
addEventListener('popstate', () => { selectedID = new URL(location.href).searchParams.get('request_id'); selectedContext = undefined; renderList(); renderDetail(); });
document.addEventListener('visibilitychange', () => { if (!document.hidden) refresh(); });
setInterval(() => { if (!document.hidden) refresh(); }, 3000);
refresh();
