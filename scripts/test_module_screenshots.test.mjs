import assert from 'node:assert/strict';
import test from 'node:test';

import { waitForPage } from './test_module_screenshots.mjs';

const url = 'http://127.0.0.1:8125/';
const quickRetry = { timeoutMs: 40, retryDelayMs: 1 };

function fakeBrowser(attempts) {
  const calls = { newPage: [], media: [], navigation: [], body: [], close: 0 };
  let currentAttempt;
  const page = {
    async emulateMedia(options) {
      calls.media.push(options);
    },
    async goto(target, options) {
      currentAttempt = attempts[Math.min(calls.navigation.length, attempts.length - 1)];
      calls.navigation.push({ url: target, ...options });
      if (currentAttempt.navigationError) throw new Error(currentAttempt.navigationError);
      if (currentAttempt.backgroundRequestPending && options.waitUntil === 'networkidle') {
        throw new Error('background request never becomes idle');
      }
      return currentAttempt.status === null ? null : { status: () => currentAttempt.status ?? 200 };
    },
    locator(selector) {
      return {
        async innerText(options) {
          calls.body.push({ selector, ...options });
          return currentAttempt.bodyText ?? '';
        },
      };
    },
    async close() {
      calls.close += 1;
    },
  };
  const browser = {
    async newPage(options) {
      calls.newPage.push(options);
      return page;
    },
  };
  return { browser, page, calls };
}

test('a loaded document is ready while a background request remains pending', async () => {
  const attempt = { status: 200, bodyText: 'After Hours', backgroundRequestPending: true };
  const { browser, page, calls } = fakeBrowser([attempt]);

  assert.equal(await waitForPage(browser, url, 'After Hours', quickRetry), page);
  assert.equal(attempt.backgroundRequestPending, true);
  assert.equal(calls.navigation.length, 1);
  assert.equal(calls.navigation[0].url, url);
  assert.equal(calls.navigation[0].waitUntil, 'load');
  assert.equal(calls.body[0].selector, 'body');
  assert.deepEqual(calls.newPage, [{
    viewport: { width: 1440, height: 1000 },
    locale: 'en-US',
    timezoneId: 'Europe/Amsterdam',
  }]);
  assert.deepEqual(calls.media, [{ reducedMotion: 'reduce' }]);
  assert.equal(calls.close, 0, 'the caller still needs the successful page for its screenshot');
});

for (const status of [404, 503, null]) {
  test(`an empty marker does not accept ${status === null ? 'a missing response' : `HTTP ${status}`}`, async () => {
    const { browser, calls } = fakeBrowser([{ status, bodyText: 'Error page' }]);
    const expected = `expected HTTP 200, got ${status === null ? 'no response' : status}`;

    await assert.rejects(waitForPage(browser, url, '', quickRetry), (error) => {
      assert.ok(error.message.includes(`timed out waiting for ${url}`));
      assert.ok(error.message.includes(expected));
      return true;
    });
    assert.ok(calls.navigation.length >= 1);
    assert.equal(calls.body.length, 0, 'error documents must not satisfy readiness');
    assert.equal(calls.close, 1, 'a failed readiness check must close its page');
  });
}

test('HTTP 200 with an empty marker accepts an empty document body', async () => {
  const { browser, page, calls } = fakeBrowser([{ status: 200, bodyText: '' }]);

  assert.equal(await waitForPage(browser, url, '', quickRetry), page);
  assert.equal(calls.navigation.length, 1);
  assert.equal(calls.body.length, 1);
  assert.equal(calls.close, 0);
});

test('HTTP 200 must still contain the expected page marker', async () => {
  const { browser, calls } = fakeBrowser([{ status: 200, bodyText: 'A different page' }]);

  await assert.rejects(waitForPage(browser, url, 'After Hours', quickRetry), (error) => {
    assert.ok(error.message.includes(`timed out waiting for ${url} (After Hours)`));
    assert.ok(error.message.includes('missing page text "After Hours"'));
    return true;
  });
  assert.ok(calls.body.length >= 1);
  assert.equal(calls.close, 1);
});

test('timeout diagnostics describe the latest attempt instead of an earlier navigation failure', async () => {
  const { browser, calls } = fakeBrowser([
    { navigationError: 'connection refused during startup' },
    { status: 200, bodyText: 'The wrong page' },
  ]);

  await assert.rejects(waitForPage(browser, url, 'Tasks', quickRetry), (error) => {
    assert.ok(error.message.includes('missing page text "Tasks"'));
    assert.ok(!error.message.includes('connection refused during startup'));
    return true;
  });
  assert.ok(calls.navigation.length >= 2);
  assert.ok(calls.body.length >= 1);
  assert.equal(calls.close, 1);
});

test('a transient navigation failure is retried and leaves the successful page open', async () => {
  const { browser, page, calls } = fakeBrowser([
    { navigationError: 'connection refused during startup' },
    { status: 200, bodyText: 'Tasks are ready' },
  ]);

  assert.equal(await waitForPage(browser, url, 'Tasks', quickRetry), page);
  assert.equal(calls.newPage.length, 1);
  assert.equal(calls.navigation.length, 2);
  assert.equal(calls.body.length, 1);
  assert.equal(calls.close, 0);
});
