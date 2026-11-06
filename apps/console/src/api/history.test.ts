import { describe, expect, it, vi } from 'vitest';
import { HISTORY_LIMITATIONS, parseCapabilities, parseHistoricalActivity, SCHEMA } from './contracts';
import { endpoints, loadHistoricalActivity } from './client';

const fixture = () => ({
  schema_version: SCHEMA, source: 'approved_detached_historical_report_only',
  activity_status: 'historical_records_not_run_history', security_verdict: 'inconclusive',
  records: [
    ['snapshot', 'kubernetes_snapshot', '2026-11-06T00:18:01Z', 'Read-only Kubernetes observation', 'historical_not_current'],
    ['plan', 'terraform_plan', '2026-11-05T17:22:35Z', 'Speculative Terraform plan receipt', 'plan_not_applied'],
    ['witness-data-to-app', 'historical_witness', '2026-11-05T07:08:05Z', 'data-to-app A/B/A historical observation', 'historical_not_causal'],
    ['witness-edge-to-data', 'historical_witness', '2026-11-05T06:56:50Z', 'edge-to-data A/B/A historical observation', 'historical_not_causal'],
  ].map(([record_id, kind, captured_utc, subject, evidence_class]) => ({
    record_id, kind, captured_utc, subject, evidence_class, execution_verified: false,
  })),
  drift: { status: 'not_assessed', comparable_snapshots: 0, current_inventory: false },
  limitations: [...HISTORY_LIMITATIONS] as string[],
});
const copy = <T>(x: T): T => JSON.parse(JSON.stringify(x)) as T;

describe('6D3A minimized, approved historical timeline', () => {
  it('accepts four historical entries and never asserts runs, causal proof or drift', () => {
    const parsed = parseHistoricalActivity(fixture());
    expect(parsed.records).toHaveLength(4);
    expect(parsed.records.every((r) => !r.execution_verified)).toBe(true);
    expect(parsed.drift.status).toBe('not_assessed');
    expect(parsed.drift.comparable_snapshots).toBe(0);
  });
  it('rejects fabricated drift/execution claims, raw data, altered chronology and unsupported sources', () => {
    for (const mutate of [
      (x: ReturnType<typeof fixture>) => { x.records[0].execution_verified = true; },
      (x: ReturnType<typeof fixture>) => { x.records[1].captured_utc = '2026-11-07T00:00:00Z'; },
      (x: ReturnType<typeof fixture>) => { x.drift.status = 'detected'; },
      (x: ReturnType<typeof fixture>) => { x.drift.current_inventory = true; },
      (x: ReturnType<typeof fixture>) => { x.security_verdict = 'contained'; },
      (x: ReturnType<typeof fixture>) => { x.records[3].kind = 'enforcement_run'; },
      (x: ReturnType<typeof fixture>) => { x.records[2].record_id = 'witness-edge-to-data'; },
      (x: ReturnType<typeof fixture>) => { x.limitations[0] = 'Enforcement confirmed'; },
    ]) { const value = copy(fixture()); mutate(value); expect(() => parseHistoricalActivity(value)).toThrow(); }
    expect(() => parseHistoricalActivity({ ...fixture(), terraform_state: { password: 'bad' } })).toThrow();
    expect(() => parseHistoricalActivity({ ...fixture(), records: [...fixture().records, { ...fixture().records[0] }] })).toThrow();
  });
  it('accepts history capability only in approved mode', () => {
    const caps = {
      schema_version: SCHEMA, mode: 'offline_contracts_only', limitations: ['Offline'],
      capabilities: [
        { name: 'health', path: '/api/v1/health', availability: 'available' },
        { name: 'scenario_contracts', path: '/api/v1/scenarios', availability: 'available' },
        { name: 'proof_reports', availability: 'not_enabled' },
        { name: 'control_findings', availability: 'not_enabled' },
        { name: 'environment_inventory', availability: 'not_enabled' },
        { name: 'scenario_execution', availability: 'not_enabled' },
        { name: 'control_catalog', path: '/api/v1/control-catalog', availability: 'available' },
        { name: 'historical_environment_snapshot', availability: 'not_enabled' },
        { name: 'historical_activity', availability: 'not_enabled' },
      ],
    };
    expect(parseCapabilities(caps).capabilities[8].availability).toBe('not_enabled');
    const enabled = copy(caps);
    enabled.capabilities[8].availability = 'available';
    enabled.capabilities[8].path = '/api/v1/historical-activity';
    expect(() => parseCapabilities(enabled)).toThrow();
    enabled.mode = 'approved_detached_historical_report_read_only';
    enabled.capabilities[2].availability = 'available'; enabled.capabilities[2].path = '/api/v1/proof-summary';
    enabled.capabilities[7].availability = 'available'; enabled.capabilities[7].path = '/api/v1/environment-snapshot';
    expect(parseCapabilities(enabled).capabilities[8].availability).toBe('available');
  });
  it('uses bounded, credential-free GET for opt-in history', async () => {
    const spy = vi.spyOn(globalThis, 'fetch');
    try {
      spy.mockResolvedValueOnce(new Response(JSON.stringify(fixture()), { headers: { 'Content-Type': 'application/json' } }));
      expect((await loadHistoricalActivity(new AbortController().signal)).records).toHaveLength(4);
      expect(spy).toHaveBeenCalledWith(endpoints.history,
        expect.objectContaining({ method: 'GET', cache: 'no-store', credentials: 'omit', redirect: 'error' }));
      spy.mockResolvedValueOnce(new Response(JSON.stringify({ ...fixture(), extra: 'x'.repeat(40000) }), { headers: { 'Content-Type': 'application/json' } }));
      await expect(loadHistoricalActivity(new AbortController().signal)).rejects.toThrow();
    } finally { spy.mockRestore(); }
  });
});
