#!/usr/bin/env node
/**
 * Firedoor G3B: explicit, local-only historical proof screenshots.
 * Node 22+ / Chrome or Edge: no npm browser dependencies, no Playwright installation.
 * Does not run Terraform, Kubernetes, probes, or servers. Never writes into the repo.
 * No live verification is claimed. Never place the private F5 receipt here.
 */
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { existsSync } from 'node:fs';
import { mkdtemp, readFile, rm, stat, writeFile, mkdir } from 'node:fs/promises';
import { homedir, tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const BASE = 'http://127.0.0.1:5173/';
const HEALTH = 'http://127.0.0.1:8787/api/v1/health';
const TIMEOUT_MS = 30000;
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..');
const SHA = (bytes) => createHash('sha256').update(bytes).digest('hex').toUpperCase();
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
function fail(message) { throw new Error(message); }
function cli(args) {
  const allowed = new Set(['--proof', '--sha256', '--out', '--chrome']);
  if (args.length === 1 && args[0] === '--self-test') return { selfTest: true };
  if (args.length % 2 !== 0) fail('Expected --proof PATH --sha256 SHA256 --out NEW_DIRECTORY [--chrome EXE].');
  const options = {};
  for (let i = 0; i < args.length; i += 2) {
    const flag = args[i];
    if (!allowed.has(flag) || flag in options || !args[i + 1] || args[i + 1].startsWith('--')) fail('Invalid or duplicate CLI option.');
    options[flag] = args[i + 1];
  }
  if (!options['--proof'] || !options['--sha256'] || !options['--out']) fail('Proof path, SHA256 and output directory are mandatory.');
  if (!/^[a-fA-F0-9]{64}$/.test(options['--sha256'])) fail('Expected a full 64-digit SHA256.');
  const proof = path.resolve(options['--proof']);
  const out = path.resolve(options['--out']);
  const chrome = options['--chrome'] ? path.resolve(options['--chrome']) : null;
  const relative = path.relative(ROOT, out);
  if (!relative.startsWith('..') && !path.isAbsolute(relative)) fail('Screenshot output must be outside the Firedoor repository.');
  if (out === path.parse(out).root || out === homedir()) fail('Refusing broad output directory.');
  return { proof, out, chrome, expected: options['--sha256'].toUpperCase() };
}
function pickBrowser(explicit) {
  const pf = process.env.PROGRAMFILES || 'C:\\Program Files';
  const pf86 = process.env['PROGRAMFILES(X86)'] || 'C:\\Program Files (x86)';
  const lad = process.env.LOCALAPPDATA || '';
  const choices = [explicit,
    path.join(pf, 'Google', 'Chrome', 'Application', 'chrome.exe'),
    path.join(pf86, 'Google', 'Chrome', 'Application', 'chrome.exe'),
    path.join(lad, 'Google', 'Chrome', 'Application', 'chrome.exe'),
    path.join(pf, 'Microsoft', 'Edge', 'Application', 'msedge.exe'),
    path.join(pf86, 'Microsoft', 'Edge', 'Application', 'msedge.exe'),
    '/usr/bin/chromium', '/usr/bin/google-chrome'];
  const found = choices.find((item) => item && existsSync(item));
  if (!found) fail('Chrome or Edge executable not found. Pass --chrome with its absolute path.');
  return found;
}
class CDP {
  constructor(socket) {
    this.socket = socket;
    this.seq = 0;
    this.pending = new Map();
    socket.addEventListener('message', (event) => {
      let data;
      try { data = JSON.parse(String(event.data)); } catch { return; }
      if (!data.id || !this.pending.has(data.id)) return;
      const pending = this.pending.get(data.id);
      this.pending.delete(data.id);
      clearTimeout(pending.timer);
      if (data.error) pending.reject(new Error(`${pending.method}: ${data.error.message}`));
      else pending.resolve(data.result ?? {});
    });
    socket.addEventListener('close', () => this.rejectAll('Browser debugging connection closed.'));
    socket.addEventListener('error', () => this.rejectAll('Browser debugging connection failed.'));
  }
  rejectAll(message) {
    for (const p of this.pending.values()) { clearTimeout(p.timer); p.reject(new Error(message)); }
    this.pending.clear();
  }
  send(method, params = {}, sessionId) {
    const id = ++this.seq;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => { this.pending.delete(id); reject(new Error(`${method}: timed out.`)); }, TIMEOUT_MS);
      this.pending.set(id, { resolve, reject, timer, method });
      try { this.socket.send(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) })); }
      catch (err) { clearTimeout(timer); this.pending.delete(id); reject(err); }
    });
  }
  async evaluate(sessionId, expression) {
    const result = await this.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true, userGesture: true }, sessionId);
    if (result.exceptionDetails) fail(`Browser UI operation failed: ${result.exceptionDetails.text}`);
    return result.result?.value;
  }
}
async function waitFor(check, label, ms = 18000) {
  const until = Date.now() + ms;
  while (Date.now() < until) {
    try { if (await check()) return; } catch { /* asynchronous UI render */ }
    await sleep(150);
  }
  fail(`Timed out waiting for ${label}.`);
}
async function connect(wsURL) {
  if (typeof WebSocket !== 'function') fail('Node 22+ WebSocket support required.');
  const socket = new WebSocket(wsURL);
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('Chrome debugger websocket timeout.')), 15000);
    socket.addEventListener('open', () => { clearTimeout(timer); resolve(); }, { once: true });
    socket.addEventListener('error', () => { clearTimeout(timer); reject(new Error('Cannot attach to isolated Chrome debugger.')); }, { once: true });
  });
  return socket;
}
async function preflight(options) {
  const item = await stat(options.proof);
  if (!item.isFile() || item.size < 100 || item.size > 32 * 1024) fail('Expected a sanitized G1 JSON file smaller than 32 KiB.');
  const bytes = await readFile(options.proof);
  const actual = SHA(bytes);
  if (actual !== options.expected) fail(`Proof SHA256 mismatch. Expected ${options.expected}; got ${actual}. Stop.`);
  let parsed;
  try { parsed = JSON.parse(bytes.toString('utf8').replace(/^\uFEFF/, '')); }
  catch { fail('Proof JSON is not parseable.'); }
  if (parsed?.schema_version !== 'firedoor.v10.console-live-proof-projection.v1' ||
      parsed?.current_cluster_checked !== false || parsed?.current_enforcement_verified !== false ||
      parsed?.allowed_count !== 3 || parsed?.denied_count !== 2 || !Array.isArray(parsed?.observations) || parsed.observations.length !== 5) {
    fail('Proof does not have the expected historical-only G1 shape.');
  }
  if (existsSync(options.out)) fail('Output directory already exists. Refusing overwrite.');
  // The app itself independently performs strict G1 schema validation after local browser import.
  for (const url of [BASE, HEALTH]) {
    const response = await fetch(url, { method: 'GET', signal: AbortSignal.timeout(5000), redirect: 'error' });
    if (!response.ok) fail(`Required local service not ready: ${url}, HTTP ${response.status}`);
    if (url === HEALTH) {
      const data = await response.json();
      if (data?.status !== 'ok' || data?.service !== 'firedoor-console-api') fail('Unexpected local Go API health response.');
    }
  }
  return { bytes, actual };
}
async function capture(options) {
  const { actual } = await preflight(options);
  const executable = pickBrowser(options.chrome);
  const profile = await mkdtemp(path.join(tmpdir(), 'firedoor-g3b-chrome-'));
  let child;
  let socket;
  let cdp;
  try {
    const flags = [
      '--headless=new', '--no-first-run', '--no-default-browser-check',
      '--disable-background-networking', '--disable-extensions', '--disable-sync', '--no-proxy-server',
      '--disable-features=Translate', '--disable-popup-blocking', '--mute-audio',
      '--remote-debugging-address=127.0.0.1', '--remote-debugging-port=0',
      `--user-data-dir=${profile}`, '--window-size=1440,900', 'about:blank',
    ];
    if (process.platform !== 'win32' && process.getuid?.() === 0) flags.unshift('--no-sandbox');
    child = spawn(executable, flags, { stdio: ['ignore', 'ignore', 'pipe'], windowsHide: true });
    let stderr = '';
    child.stderr?.on('data', (data) => { stderr = (stderr + String(data)).slice(-4000); });
    const debugFile = path.join(profile, 'DevToolsActivePort');
    await waitFor(async () => existsSync(debugFile) || child.exitCode !== null, 'isolated browser startup', 25000);
    if (!existsSync(debugFile)) fail(`Isolated browser did not start: ${stderr.slice(-800)}`);
    const lines = (await readFile(debugFile, 'utf8')).trim().split(/\r?\n/);
    if (!/^\d{2,5}$/.test(lines[0]) || !/^\/devtools\/browser\//.test(lines[1])) fail('Invalid isolated browser debugging endpoint.');
    const wsURL = `ws://127.0.0.1:${lines[0]}${lines[1]}`;
    socket = await connect(wsURL);
    cdp = new CDP(socket);
    const target = await cdp.send('Target.createTarget', { url: 'about:blank' });
    const attached = await cdp.send('Target.attachToTarget', { targetId: target.targetId, flatten: true });
    const session = attached.sessionId;
    await cdp.send('Page.enable', {}, session);
    await cdp.send('Runtime.enable', {}, session);
    await cdp.send('DOM.enable', {}, session);
    const viewport = async (width, height) => cdp.send('Emulation.setDeviceMetricsOverride', {
      width, height, deviceScaleFactor: 1, mobile: false, screenWidth: width, screenHeight: height,
    }, session);
    await viewport(1440, 900);
    await cdp.send('Page.navigate', { url: BASE }, session);
    await waitFor(() => cdp.evaluate(session, `document.querySelectorAll('nav[aria-label="Main navigation"] button').length >= 7`), 'Firedoor nav');
    await cdp.evaluate(session, `Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] button')).find(x=>x.textContent.trim()==='Live Proof')?.click()`);
    await waitFor(() => cdp.evaluate(session, `!!document.querySelector('.lp-empty')`), 'empty historical proof view');
    await cdp.evaluate(session, `document.fonts.ready.then(() => true)`);
    await mkdir(options.out, { recursive: false });
    const screenshots = [];
    const shot = async (name, label) => {
      await sleep(200);
      const result = await cdp.send('Page.captureScreenshot', { format: 'png', fromSurface: true, captureBeyondViewport: false }, session);
      const bytes = Buffer.from(result.data ?? '', 'base64');
      if (bytes.length < 10000 || bytes.toString('ascii', 1, 4) !== 'PNG') fail(`Invalid screenshot: ${name}`);
      await writeFile(path.join(options.out, name), bytes, { flag: 'wx' });
      screenshots.push({ file: name, view: label, sha256: SHA(bytes) });
      console.log(`PASS: ${name} (${bytes.length} bytes)`);
    };
    await shot('01-empty-historical-proof.png', 'no evidence loaded');
    const doc = await cdp.send('DOM.getDocument', { depth: -1, pierce: true }, session);
    const input = await cdp.send('DOM.querySelector', { nodeId: doc.root.nodeId, selector: '#lp-proof-file' }, session);
    if (!input.nodeId) fail('Strict G1 file input missing.');
    await cdp.send('DOM.setFileInputFiles', { nodeId: input.nodeId, files: [options.proof] }, session);
    await waitFor(() => cdp.evaluate(session, `!!document.querySelector('.lp-stat-grid') && document.querySelector('.lp-file-status')?.textContent?.startsWith('Loaded: ')`), 'explicitly imported historical proof');
    const safe = await cdp.evaluate(session, `(() => {
      const stats = document.querySelector('.lp-stat-grid')?.textContent || '';
      const capture = document.querySelector('.lp-capture')?.textContent || '';
      const routes = [...document.querySelectorAll('.lp-route')];
      return routes.length===5 && stats.includes('Current live check') && stats.includes('NO') &&
        stats.includes('Allowed at capture') && stats.includes('Blocked at capture') &&
        capture.includes('Captured, not current.') && !document.querySelector('.lp-error');
    })()`);
    if (!safe) fail('Browser rejected sanitized proof or rendered an unexpected enforcement claim.');
    await shot('02-loaded-proof-overview.png', 'historical proof loaded, no live check');
    const selectRoute = async (name, expected) => {
      const clicked = await cdp.evaluate(session, `(() => { const el=[...document.querySelectorAll('.lp-route')].find(b=>b.querySelector('.lp-route-name')?.textContent.trim()===${JSON.stringify(name)}); if (!el) return false; el.click(); return true; })()`);
      if (!clicked) fail(`Route missing: ${name}`);
      await waitFor(() => cdp.evaluate(session, `document.querySelector('.lp-detail h2')?.textContent?.trim()===${JSON.stringify(name)} && document.querySelector('.lp-detail-status')?.textContent?.toLowerCase().includes(${JSON.stringify(expected)})`), `selected ${name}`);
      await cdp.evaluate(session, `document.querySelector('.lp-report-grid')?.scrollIntoView({block:'start',behavior:'instant'})`);
    };
    await selectRoute('App → Data', 'allowed');
    await shot('03-allowed-app-data.png', 'historical allowed path detail');
    await selectRoute('Data → App', 'blocked');
    await shot('04-blocked-data-app.png', 'historical blocked path detail');
    await cdp.evaluate(session, `document.querySelector('.lp-limitations')?.scrollIntoView({block:'end',behavior:'instant'})`);
    await shot('05-evidence-limitations.png', 'historical provenance and limitations');
    await viewport(430, 900);
    await cdp.evaluate(session, `window.scrollTo(0,0)`);
    await shot('06-mobile-historical-proof.png', 'mobile viewport, evidence remains historical');
    const manifest = {
      schema_version: 'firedoor.v10.local-screenshot-manifest.v1',
      provenance: 'user-selected sanitized G1 historical projection',
      proof_sha256: actual,
      current_cluster_checked: false,
      current_enforcement_verified: false,
      no_infrastructure_commands: true,
      capture_utc: new Date().toISOString(),
      screenshots,
    };
    await writeFile(path.join(options.out, 'capture-manifest.json'), JSON.stringify(manifest, null, 2) + '\n', { flag: 'wx' });
    console.log('PASS: Six browser screenshots saved. Historical-only scope preserved.');
    console.log(`OUTPUT: ${options.out}`);
    console.log('No Terraform, Kubernetes, network probes, or private receipt reads.');
  } finally {
    try { cdp?.rejectAll('Closing capture browser.'); socket?.close(); } catch { /* cleanup */ }
    if (child && child.exitCode === null) {
      child.kill();
      await Promise.race([new Promise((resolve) => child.once('exit', resolve)), sleep(1800)]);
    }
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
  }
}
function selfTest() {
  const x = cli(['--proof', path.join(tmpdir(), 'p.json'), '--sha256', 'a'.repeat(64), '--out', path.join(tmpdir(), 'fd-g3b-check')]);
  if (x.expected !== 'A'.repeat(64)) fail('SHA normalization self-test failed.');
  for (const bad of [[], ['--proof', 'x'], ['--proof', 'x', '--proof', 'y', '--sha256', 'a'.repeat(64), '--out', 'z']]) {
    let rejected = false;
    try { cli(bad); } catch { rejected = true; }
    if (!rejected) fail('CLI negative self-test failed.');
  }
  if (SHA(Buffer.from('abc')) !== 'BA7816BF8F01CFEA414140DE5DAE2223B00361A396177A9CB410FF61F20015AD') fail('SHA256 self-test failed.');
  console.log('PASS: G3B CLI, SHA256 and negative self-tests. No network, files or browser used.');
}
try {
  const options = cli(process.argv.slice(2));
  if (options.selfTest) selfTest();
  else await capture(options);
} catch (err) {
  console.error(`STOP: ${err instanceof Error ? err.message : String(err)}`);
  process.exitCode = 1;
}
