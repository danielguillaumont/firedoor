import { afterEach, describe, expect, it, vi } from 'vitest';
import { endpoints, loadApprovedHistoricalContainmentMap } from './client';
import { isHistoricalMapApproved, parseHistoricalContainmentMap } from './containmentHistorical';
import { historicalFixture as fixture, historicalStamp } from './containmentHistorical.fixtures';
import type { Capabilities } from './contracts';

const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value)) as T;
const response = (body: unknown) => new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } });
const signal = () => new AbortController().signal;

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers(); });

describe('7E2B strict separately approved historical route contract', () => {
  it('accepts scoped historical measurements with inconclusive—not verified—assessment', () => {
    for (const profile of ['baseline', 'hardened'] as const) {
      const parsed = parseHistoricalContainmentMap(fixture(profile));
      expect(parsed.profile).toBe(profile);
      expect(parsed.observed_count).toBe(2);
      expect(parsed.routes).toHaveLength(6);
      expect(parsed.routes[0].observed).toBe('reachable');
      expect(parsed.routes[0].assessment).toBe('inconclusive');
      expect(parsed.routes[0].captured_utc).toBe(historicalStamp);
      expect(parsed.routes[2].assessment).toBe('not_assessed');
      expect(parsed.routes[2].captured_utc).toBeNull();
      expect(JSON.stringify(parsed)).not.toContain('source_id');
    }
  });
  it('allows zero reliable observations without inventing a finding', () => {
    const data = fixture();
    data.routes.forEach((route) => {
      route.observed = 'not_observed'; route.assessment = 'not_assessed';
      route.reason = 'Saved v0.3 probe did not establish reliable reachability. This route remains unobserved and not assessed.';
      delete (route as { observation_evidence_ids?: string[] }).observation_evidence_ids;
    });
    data.evidence = [];
    const parsed = parseHistoricalContainmentMap(data);
    expect(parsed.observed_count).toBe(0);
    expect(parsed.routes.every((route) => route.assessment === 'not_assessed')).toBe(true);
  });
  it('rejects injected raw metadata, unknown cluster scope, and false enforcement or current inventory', () => {
    for (const mutation of [
      { source: 'declared_contract' }, { schema_version: 'unreviewed' }, { profile: 'production' },
      { cluster: 'prod' }, { context: 'prod' }, { port: 443 }, { endpoint_mode: 'service-dns' },
      { current_inventory: true }, { enforcement_verified: true }, { raw_suite_path: 'C:/private/secret.json' },
    ]) expect(() => parseHistoricalContainmentMap({ ...fixture(), ...mutation })).toThrow();
  });
  it('rejects extra node and route fields, duplicate routes, missing routes and remapped zones', () => {
    const raw = fixture();
    const bad = clone(raw); Object.assign(bad.nodes[0], { pod_ip: '10.1.1.1' });
    expect(() => parseHistoricalContainmentMap(bad)).toThrow();
    const extra = clone(raw); Object.assign(extra.routes[0], { policy_verified: true });
    expect(() => parseHistoricalContainmentMap(extra)).toThrow();
    const repeated = clone(raw); repeated.routes[1] = clone(repeated.routes[0]);
    expect(() => parseHistoricalContainmentMap(repeated)).toThrow();
    const short = clone(raw); short.routes.pop();
    expect(() => parseHistoricalContainmentMap(short)).toThrow();
    const mapped = clone(raw); mapped.routes[0].to = 'data';
    expect(() => parseHistoricalContainmentMap(mapped)).toThrow();
  });
  it('rejects forged observations, assessed unknown routes, mismatched references, origin and reason', () => {
    const wrong = clone(fixture()); wrong.routes[0].assessment = 'pass';
    expect(() => parseHistoricalContainmentMap(wrong)).toThrow();
    const absent = clone(fixture()); absent.routes[2].observed = 'reachable';
    expect(() => parseHistoricalContainmentMap(absent)).toThrow();
    const ref = clone(fixture()); ref.routes[0].observation_evidence_ids = ['observation-data-to-edge'];
    expect(() => parseHistoricalContainmentMap(ref)).toThrow();
    const origin = clone(fixture()); origin.evidence[0].source_id = 'raw-private-hash';
    expect(() => parseHistoricalContainmentMap(origin)).toThrow();
    const reason = clone(fixture()); reason.routes[0].reason = 'Proven policy enforcement';
    expect(() => parseHistoricalContainmentMap(reason)).toThrow();
    const repeat = clone(fixture()); repeat.evidence.push(clone(repeat.evidence[0]));
    expect(() => parseHistoricalContainmentMap(repeat)).toThrow();
  });
  it('refuses missing or invalid UTC timestamps and inconsistent evidence', () => {
    for (const value of ['today', '2026-11-07T12:00:00-05:00', 'invalid', '0001-01-01T00:00:00Z', '2026-02-31T12:00:00Z']) {
      const data = clone(fixture()); data.evidence[0].captured_at_utc = value;
      expect(() => parseHistoricalContainmentMap(data)).toThrow();
    }
    const conflicting = clone(fixture()); conflicting.evidence[0].result = 'unreachable';
    expect(() => parseHistoricalContainmentMap(conflicting)).toThrow();
    const extra = clone(fixture()); (extra.limitations as string[]).push('Claim containment achieved');
    expect(() => parseHistoricalContainmentMap(extra)).toThrow();
    expect(() => parseHistoricalContainmentMap(null)).toThrow();
  });
  it('only considers explicitly advertised approved historical map capability', () => {
    const make = (mode: Capabilities['mode'], available: boolean): Capabilities => ({
      schema_version: 'firedoor.v06.console-api.v1', mode,
      capabilities: [{ name: 'historical_containment_map', availability: available ? 'available' : 'not_enabled',
        ...(available ? { path: '/api/v1/containment-map/historical' } : {}) }], limitations: [],
    });
    expect(isHistoricalMapApproved(null)).toBe(false);
    expect(isHistoricalMapApproved(make('offline_contracts_only', false))).toBe(false);
    expect(isHistoricalMapApproved(make('approved_detached_historical_report_read_only', false))).toBe(false);
    expect(isHistoricalMapApproved(make('approved_v07_historical_map_read_only', true))).toBe(true);
    expect(isHistoricalMapApproved(make('approved_v07_historical_map_read_only', false))).toBe(false);
  });
  it('uses the bounded credential-free GET transport and fails closed on HTTP error or malformed JSON', async () => {
    const spy = vi.spyOn(globalThis, 'fetch');
    spy.mockResolvedValueOnce(response(fixture()));
    const result = await loadApprovedHistoricalContainmentMap(signal());
    expect(result.observed_count).toBe(2);
    expect(spy).toHaveBeenCalledWith(endpoints.historicalMap, expect.objectContaining({
      method: 'GET', credentials: 'omit', cache: 'no-store', redirect: 'error', headers: { Accept: 'application/json' },
    }));
    spy.mockResolvedValueOnce(new Response('Not Found', { status: 404 }));
    await expect(loadApprovedHistoricalContainmentMap(signal())).rejects.toThrow(/HTTP 404/);
    spy.mockResolvedValueOnce(response({ ...fixture(), raw_data: 'private' }));
    await expect(loadApprovedHistoricalContainmentMap(signal())).rejects.toThrow();
    spy.mockResolvedValueOnce(response({ ...fixture(), padding: 'x'.repeat(40 * 1024) }));
    await expect(loadApprovedHistoricalContainmentMap(signal())).rejects.toThrow(/size limit/);
  });
  it('propagates request cancellation without supplying stale historical data', async () => {
    vi.spyOn(globalThis, 'fetch').mockImplementation((_path, options) => new Promise<Response>((_resolve, reject) => {
      options?.signal?.addEventListener('abort', () => reject(new DOMException('cancelled', 'AbortError')), { once: true });
    }));
    const controller = new AbortController();
    const pending = loadApprovedHistoricalContainmentMap(controller.signal);
    const rejection = expect(pending).rejects.toThrow();
    controller.abort();
    await rejection;
  });
});
