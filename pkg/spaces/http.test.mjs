import test from 'node:test';
import assert from 'node:assert/strict';
import {readResponse} from './web/http.mjs';

test('valid API responses are preserved', async () => {
  const value = {write:true, spaces:[]};
  assert.deepEqual(await readResponse(Response.json(value)), value);
});
test('source conflict errors preserve their message and HTTP status', async () => {
  await assert.rejects(readResponse(Response.json({error:'Source changed'}, {status:409})), {message:'Source changed', status:409});
});
test('plain-text rate limits become actionable errors, not JSON exceptions', async () => {
  await assert.rejects(readResponse(new Response('Too many requests', {status:429})), {message:'Too many requests. Wait a moment, then try again.', status:429});
});
test('invalid successful responses and HTML server failures are explicit', async () => {
  await assert.rejects(readResponse(new Response('<html>gateway error</html>', {status:502})), {message:'Request failed (502). Try again or check the server logs.', status:502});
  await assert.rejects(readResponse(new Response('not JSON')), {message:'Invalid server response. Reload source files and try again.', status:200});
});
