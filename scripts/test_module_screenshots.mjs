#!/usr/bin/env node

import { chromium } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import { spawn } from 'node:child_process';
import path from 'node:path';
import process from 'node:process';
import net from 'node:net';
import { fileURLToPath } from 'node:url';

const root = path.resolve(new URL('..', import.meta.url).pathname);
const skipPluginBuild = process.argv.includes('--skip-plugin-build');
const modules = [
  ['esbuild-demo', 8097, '/', 'VAT'],
  ['navigation-demo-swup', 8125, '/', 'After Hours'],
  ['todo-demo-htmx', 8121, '/', 'Your list is ready.'],
  ['todo-demo-swup', 8124, '/', 'Your list is ready.'],
  ['todo-demo-turbo', 8122, '/', 'Your list is ready.'],
  ['todo-demo-unpoly', 8123, '/', 'Your list is ready.'],
  ['unpoly-guard-demo', 8132, '/', 'Sign in'],
  ['hyperbricks-patterns-yaml', 8080, '/docs', 'HyperBricks Patterns'],
  ['hyperbricks-patterns-yaml', 8080, '/docs/readme', 'HyperBricks Patterns'],
  ['hyperbricks-patterns-yaml', 8080, '/docs/markdown-plugin', ''],
  ['hyperbricks-patterns-yaml', 8080, '/docs/template-config-plugin', ''],
  ['hyperbricks-patterns-yaml', 8080, '/docs/htmx-canonical-fragment-demo', ''],
  ['hyperbricks-patterns-yaml', 8080, '/docs/guarded-page-demo', ''],
  ['hyperbricks-patterns-yaml', 8080, '/docs/api-fragment-write-demo', ''],
  ['hyperbricks-patterns-yaml', 8080, '/docs/menu-htmx-demo', ''],
  ['hyperbricks-patterns-yaml', 8080, '/docs/sidebar-section-navigation', ''],
  ['hyperbricks-patterns-yaml', 8080, '/docs/single-plugin-many-actions', ''],
  ['hyperbricks-patterns-yaml', 8080, '/docs/plugin-vs-api-route-split', ''],
  ['hyperbricks-patterns-yaml', 8080, '/docs/unpoly-fragment-demo', ''],
  ['hyperbricks-patterns-yaml', 8080, '/docs/localized-spaces', ''],
];

function waitForExit(child) {
  return new Promise((resolve) => child.once('exit', resolve));
}

async function run(command, args) {
  const child = spawn(command, args, { cwd: root, env: { ...process.env, GOWORK: 'off', HYPERBRICKS_LOCAL_PATH: root }, stdio: 'inherit' });
  const code = await new Promise((resolve) => child.once('exit', resolve));
  if (code !== 0) throw new Error(`${command} ${args.join(' ')} exited with ${code}`);
}

function freePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const port = server.address().port;
      server.close(() => resolve(port));
    });
  });
}

export async function waitForPage(browser, url, marker, { timeoutMs = 30000, retryDelayMs = 250 } = {}) {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 }, locale: 'en-US', timezoneId: 'Europe/Amsterdam' });
  await page.emulateMedia({ reducedMotion: 'reduce' });
  const deadline = Date.now() + timeoutMs;
  let lastError = 'page did not become ready';
  while (Date.now() < deadline) {
    try {
      // Background requests (such as the optional Spaces API) do not determine
      // whether the document and its assets are ready for a screenshot.
      const response = await page.goto(url, { waitUntil: 'load', timeout: Math.max(1, Math.min(5000, deadline - Date.now())) });
      if (!response || response.status() !== 200) {
        throw new Error(`expected HTTP 200, got ${response ? response.status() : 'no response'}`);
      }
      while (Date.now() < deadline) {
        const body = await page.locator('body').innerText({ timeout: Math.max(1, deadline - Date.now()) });
        if (!marker || body.includes(marker)) return page;
        lastError = `missing page text ${JSON.stringify(marker)}`;
        const delay = Math.min(retryDelayMs, deadline - Date.now());
        if (delay > 0) await new Promise((resolve) => setTimeout(resolve, delay));
      }
    } catch (error) {
      lastError = error.message;
    }
    const delay = Math.min(retryDelayMs, deadline - Date.now());
    if (delay > 0) await new Promise((resolve) => setTimeout(resolve, delay));
  }
  await page.close();
  throw new Error(`timed out waiting for ${url} (${marker}): ${lastError}`);
}

async function main() {
  const browser = await chromium.launch({ headless: true });
  let failures = 0;
  try {
    if (!skipPluginBuild && modules.some(([name]) => name === 'hyperbricks-patterns-yaml')) {
      await run('bash', ['scripts/plugins/build_hyperbricks_plugins.sh']);
    }
    console.log(`Capturing ${modules.length} module screenshots...`);
    for (const [name, port, route, marker] of modules) {
      const runtimePort = await freePort();
      const env = { ...process.env, GOWORK: 'off', HYPERBRICKS_LOCAL_PATH: root };
      const child = spawn('go', ['run', './cmd/hyperbricks', 'start', '-m', name, '--port', String(runtimePort), '--non-interactive'], { cwd: root, env, stdio: ['ignore', 'pipe', 'pipe'], detached: true });
      let serverOutput = '';
      const captureOutput = (chunk) => { serverOutput = (serverOutput + chunk.toString()).slice(-8192); };
      child.stdout.on('data', captureOutput);
      child.stderr.on('data', captureOutput);
      try {
        const page = await waitForPage(browser, `http://127.0.0.1:${runtimePort}${route}`, marker);
        const slug = route === '/' ? 'home' : route.replace(/^\//, '').replaceAll('/', '-');
        const output = path.join(root, 'modules', name, 'docs', 'screenshots', `${slug}.png`);
        await mkdir(path.dirname(output), { recursive: true });
        // Chromium can paint sticky elements in the wrong position while it
        // expands the viewport for a full-page screenshot. At the top of the
        // page, static positioning is visually equivalent and avoids capturing
        // a sidebar over the documentation panel.
        await page.addStyleTag({ content: '.pattern-docs-sidebar { position: static !important; }' });
        await page.screenshot({ path: output, fullPage: true });
        await page.close();
        console.log(`PASS ${name}: ${output}`);
      } catch (error) {
        failures += 1;
        console.error(`FAIL ${name}: ${error.message}`);
        if (serverOutput.trim()) console.error(`Server output (${name}):\n${serverOutput.trim()}`);
      } finally {
        try { process.kill(-child.pid, 'SIGTERM'); } catch {}
        await Promise.race([waitForExit(child), new Promise((resolve) => setTimeout(resolve, 5000))]);
      }
    }
  } finally {
    await browser.close();
  }
  if (failures) process.exit(1);
  console.log(`All ${modules.length} module screenshots captured successfully.`);
  console.log(`Screenshots: ${path.join(root, 'modules', '<module>', 'docs', 'screenshots', '*.png')}`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  await main();
}
