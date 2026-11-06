import { afterEach, describe, expect, it, vi } from 'vitest';
import { endpoints, getJSON, loadHealth } from './client';
import { SCHEMA } from './contracts';

const raw = (value: unknown): unknown => value;
const signal = () => new AbortController().signal;
const jsonResponse = (value: unknown, contentType = 'application/json; charset=utf-8') =>
  new Response(JSON.stringify(value), { headers: { 'Content-Type': contentType } });

// A mocked response isolates the browser transport boundary; separate
// contract tests enforce the exact Go response schemas and evidence semantics.
afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe('6E1B fail-closed, bounded localhost JSON transport', () => {
  it('accepts valid process health using fixed GET, no credentials or cache', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(jsonResponse({
      schema_version: SCHEMA, service: 'firedoor-console-api', status: 'ok',
      scope: 'local_http_process_only', cluster_checked: false, evidence_loaded: false,
    }));
    expect((await loadHealth(signal())).cluster_checked).toBe(false);
    expect(fetchSpy).toHaveBeenCalledWith(endpoints.health, expect.objectContaining({
      method: 'GET', credentials: 'omit', redirect: 'error', cache: 'no-store',
      headers: { Accept: 'application/json' }, signal: expect.any(AbortSignal),
    }));
  });

  it('limits all three ordinary contract endpoints, not just approved reports', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch');
    for (const path of [endpoints.health, endpoints.capabilities, endpoints.scenarios]) {
      fetchSpy.mockResolvedValueOnce(jsonResponse({ giant: 'x'.repeat(40 * 1024) }));
      await expect(getJSON(path, raw, signal())).rejects.toThrow(/size limit/);
    }
    expect(fetchSpy).toHaveBeenCalledTimes(3);
  });

  it('rejects large chunked responses without Content-Length and cancels the stream', async () => {
    let cancelled = false;
    const response = new Response(new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(new TextEncoder().encode('x'.repeat(20 * 1024)));
        controller.enqueue(new TextEncoder().encode('y'.repeat(20 * 1024)));
      },
      cancel() { cancelled = true; },
    }), { headers: { 'Content-Type': 'application/json' } });
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(response);
    await expect(getJSON(endpoints.capabilities, raw, signal())).rejects.toThrow(/size limit/);
    expect(cancelled).toBe(true);
  });

  it('rejects malformed or excessive declared lengths before parsing any body', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch');
    for (const length of ['32769', '-1', 'Infinity', '3.14', '999999999999999999999999']) {
      fetchSpy.mockResolvedValueOnce(new Response('{}', { headers: {
        'Content-Type': 'application/json', 'Content-Length': length,
      } }));
      await expect(getJSON(endpoints.health, raw, signal())).rejects.toThrow(/Content-Length/);
    }
  });

  it('rejects misleading Content-Type headers and non-JSON HTTP responses', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch');
    for (const mediaType of ['text/plain; hint=application/json', 'application/json-unsafe', 'text/html', '']) {
      fetchSpy.mockResolvedValueOnce(jsonResponse({}, mediaType));
      await expect(getJSON(endpoints.scenarios, raw, signal())).rejects.toThrow(/did not return JSON/);
    }
    fetchSpy.mockResolvedValueOnce(new Response('unavailable', { status: 503 }));
    await expect(getJSON(endpoints.health, raw, signal())).rejects.toThrow(/HTTP 503/);
  });

  it('rejects invalid UTF-8 and malformed JSON rather than substituting values', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch');
    fetchSpy.mockResolvedValueOnce(new Response(new Uint8Array([0x7b, 0xc3, 0x28, 0x7d]), {
      headers: { 'Content-Type': 'application/json' },
    }));
    await expect(getJSON(endpoints.health, raw, signal())).rejects.toThrow();
    fetchSpy.mockResolvedValueOnce(new Response('{broken', { headers: { 'Content-Type': 'application/json' } }));
    await expect(getJSON(endpoints.health, raw, signal())).rejects.toThrow();
  });

  it('aborts unresponsive requests after eight seconds', async () => {
    vi.useFakeTimers();
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation((_path, options) =>
      new Promise<Response>((_resolve, reject) => {
        options?.signal?.addEventListener('abort', () => reject(new DOMException('cancelled', 'AbortError')), { once: true });
      }));
    const pending = getJSON(endpoints.capabilities, raw, signal());
    // Attach the rejection assertion before advancing fake timers.
    const rejection = expect(pending).rejects.toThrow();
    await vi.advanceTimersByTimeAsync(8_001);
    await rejection;
    expect((fetchSpy.mock.calls[0][1] as RequestInit).signal?.aborted).toBe(true);
  });

  it('propagates the caller abort signal and does not keep pending work alive', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation((_path, options) =>
      new Promise<Response>((_resolve, reject) => {
        options?.signal?.addEventListener('abort', () => reject(new DOMException('cancelled', 'AbortError')), { once: true });
      }));
    const caller = new AbortController();
    const pending = getJSON(endpoints.history, raw, caller.signal);
    const rejection = expect(pending).rejects.toThrow();
    caller.abort();
    await rejection;
    expect((fetchSpy.mock.calls[0][1] as RequestInit).signal?.aborted).toBe(true);
  });
});
