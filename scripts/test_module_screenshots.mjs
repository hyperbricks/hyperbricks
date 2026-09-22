#!/usr/bin/env node

import { chromium } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import { spawn } from 'node:child_process';
import path from 'node:path';
import process from 'node:process';
import net from 'node:net';

const root = path.resolve(new URL('..', import.meta.url).pathname);
const modules = [
  ['esbuild-demo', 8097, '/', 'VAT'],
  ['navigation-demo-swup', 8125, '/', 'After Hours'],
  ['todo-demo-htmx', 8121, '/', 'Tasks'],
  ['todo-demo-swup', 8124, '/', 'Tasks'],
  ['todo-demo-turbo', 8122, '/', 'Tasks'],
  ['todo-demo-unpoly', 8123, '/', 'Tasks'],
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

async function waitForPage(browser, url, marker) {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 }, locale: 'en-US', timezoneId: 'Europe/Amsterdam' });
  await page.emulateMedia({ reducedMotion: 'reduce' });
  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    try {
      await page.goto(url, { waitUntil: 'networkidle', timeout: 5000 });
      const body = await page.locator('body').innerText();
      if (!marker || body.includes(marker)) return page;
    } catch {}
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  await page.close();
  throw new Error(`timed out waiting for ${url} (${marker})`);
}

const browser = await chromium.launch({ headless: true });
let failures = 0;
try {
  if (modules.some(([name]) => name === 'hyperbricks-patterns-yaml')) {
    await run('bash', ['scripts/plugins/build_hyperbricks_plugins.sh']);
  }
  for (const [name, port, route, marker] of modules) {
    const runtimePort = await freePort();
    const env = { ...process.env, GOWORK: 'off', HYPERBRICKS_LOCAL_PATH: root };
    const child = spawn('go', ['run', './cmd/hyperbricks', 'start', '-m', name, '--port', String(runtimePort), '--non-interactive'], { cwd: root, env, stdio: 'ignore', detached: true });
    try {
      const page = await waitForPage(browser, `http://127.0.0.1:${runtimePort}${route}`, marker);
      const slug = route === '/' ? 'home' : route.replace(/^\//, '').replaceAll('/', '-');
      const output = path.join(root, 'modules', name, 'docs', 'screenshots', `${slug}.png`);
      await mkdir(path.dirname(output), { recursive: true });
      await page.screenshot({ path: output, fullPage: true });
      await page.close();
      console.log(`PASS ${name}: ${output}`);
    } catch (error) {
      failures += 1;
      console.error(`FAIL ${name}: ${error.message}`);
    } finally {
      try { process.kill(-child.pid, 'SIGTERM'); } catch {}
      await Promise.race([waitForExit(child), new Promise((resolve) => setTimeout(resolve, 5000))]);
    }
  }
} finally {
  await browser.close();
}
if (failures) process.exit(1);
