import test from 'node:test';
import assert from 'node:assert/strict';
import {severity, recordSeverity, routeLabel, validateDiagnostics, validateCurrentDiagnostics, currentSelection, sourceLocation, filterRecords, diagnosticURL, readJSON, stripANSI} from './errors-model.mjs';

const record = (overrides = {}) => ({request_id:'hb-12',route:'example',created_at:'2026-09-15T12:00:00Z',
  errors:[{err:'Cannot render <script>unsafe</script>',file:'page.hyperbricks.yaml',path:'content.hero',key:'template',type:'<TEMPLATE>'}], ...overrides});

test('warnings remain warnings unless rejected, and mixed requests remain errors', () => {
  const warning = {err:'Optional field',level:'WARNING'};
  assert.equal(severity(warning),'warning');
  assert.equal(severity({...warning,rejected:true}),'error');
  assert.equal(severity({err:'Plain error'}),'error');
  assert.equal(recordSeverity(record({errors:[warning]})),'warning');
  assert.equal(recordSeverity(record({errors:[warning,{err:'Failed'}]})),'error');
});

test('search covers route, request, source and message; severity checks every issue', () => {
  const mixed = record({errors:[...record().errors,{err:'Optional',level:'WARNING'}]});
  for (const query of ['HB-12', '/example', 'page.hyperbricks.yaml','content.hero','template','unsafe']) {
    assert.deepEqual(filterRecords([mixed],query,'all'),[mixed]);
  }
  assert.equal(filterRecords([mixed],'','warning').length,1);
  assert.equal(filterRecords([mixed],'','error').length,1);
  assert.equal(filterRecords([mixed],'missing','all').length,0);
});

test('raw JSON links encode request IDs and configuration routes have an honest label', () => {
  assert.equal(diagnosticURL('hb-12&limit=1'),'/__hyperbricks/render-diagnostics?request_id=hb-12%26limit%3D1');
  assert.equal(routeLabel('__config'),'Configuration');
  assert.equal(routeLabel('index'),'/');
  assert.equal(routeLabel('/example'),'/example');
});

test('diagnostic payload validation accepts an empty current list, rejects malformed data', () => {
  assert.deepEqual(validateDiagnostics([]),[]);
  assert.deepEqual(validateDiagnostics([record()]),[record()]);
  for (const value of [null,{},[null],[record({errors:[]})],[record({errors:[{}]})],
    [record({created_at:'invalid'})],[record({request_id:''})],[record({route:null})]]) {
    assert.throws(()=>validateDiagnostics(value),/invalid diagnostics response/);
  }
});

test('current snapshot separates checked routes from unverified routes and rejects mixed generations', () => {
  const snapshot = {records:[record({context_id:'ctx-a',generation:2})], generation:2, checked_routes:1, total_routes:2, unchecked_routes:['other'], evicted_contexts:0};
  assert.deepEqual(validateCurrentDiagnostics(snapshot),snapshot);
  for (const value of [{}, {...snapshot,generation:3}, {...snapshot,checked_routes:3}, {...snapshot,unchecked_routes:[]}, {...snapshot,records:[record()]}]) {
    assert.throws(() => validateCurrentDiagnostics(value),/invalid .*diagnostics response/);
  }
  assert.deepEqual(validateCurrentDiagnostics({...snapshot,records:[]}).records,[]);
});

test('selection follows a repeated failure in the same context, but never a different variant', () => {
  const latest = record({request_id:'hb-20',context_id:'ctx-a'});
  const other = record({request_id:'hb-21',context_id:'ctx-b'});
  assert.equal(currentSelection([latest,other],'hb-12','ctx-a'),latest);
  assert.equal(currentSelection([other],'hb-12','ctx-a'),undefined);
  assert.equal(currentSelection([latest,other],'hb-21',undefined),other);
});

test('source and execution positions omit unknown coordinates', () => {
  assert.equal(sourceLocation('hyperbricks/page.hyperbricks.yaml',8,3),'hyperbricks/page.hyperbricks.yaml:8:3');
  assert.equal(sourceLocation('templates/card.html',2,0),'templates/card.html:2');
  assert.equal(sourceLocation('templates/card.html',0,0),'templates/card.html');
  assert.equal(sourceLocation('',0,0),'Not provided');
  assert.equal(sourceLocation('Unknown',0,0),'Not provided');
});

test('search includes resource and failure phase; information is not an error', () => {
  const item = record({errors:[{err:'Note',level:'INFO',resource:'resources/calculator.js',phase:'prepare'}]});
  assert.equal(recordSeverity(item),'info');
  assert.equal(severity({...item.errors[0],rejected:true}),'error');
  for (const query of ['calculator.js','prepare']) assert.deepEqual(filterRecords([item],query,'info'),[item]);
});

test('fetch failures have actionable messages and HTML is not treated as JSON', async () => {
  await assert.rejects(readJSON('/diagnostics',async()=>{throw new TypeError('Failed to fetch');}),/Cannot reach the runtime/);
  for (const [status, expected] of [[404,/development or debug/],[429,/Wait a moment/],[503,/HTTP 503/]]) {
    await assert.rejects(readJSON('/diagnostics',async()=>new Response('failure',{status})),expected);
  }
  await assert.rejects(readJSON('/diagnostics',async()=>new Response('<html>wrong response</html>')),/unreadable response/);
  assert.deepEqual(await readJSON('/diagnostics',async(url,options)=>{
    assert.equal(options.cache,'no-store');
    return Response.json([record()]);
  }),[record()]);
});

test('terminal escapes are removed while message contents remain plain text', () => {
  assert.equal(stripANSI('\x1b[31m<script>bad</script>\x1b[0m'),'<script>bad</script>');
});
