import { describe, expect, it } from 'vitest';
import { HISTORICAL_PROOF_LIMITATIONS, HISTORICAL_PROOF_PATHS, HISTORICAL_LIVE_PROOF_SCHEMA,
  parseHistoricalLiveProof, parseHistoricalLiveProofFile } from './historicalLiveProof';

const fixture = () => ({
  schema_version: HISTORICAL_LIVE_PROOF_SCHEMA,
  source: 'explicit_private_f5_receipt_projection',
  evidence_status: 'historical_verified_at_capture_not_live',
  lab_label: 'Disposable KIND network-policy lab',
  observed_utc: '2026-11-12T03:27:30.2363946Z',
  allowed_count: 3, denied_count: 2,
  network_verified_at_capture: true,
  current_cluster_checked: false, current_enforcement_verified: false,
  terraform_executed_by_projection: false, kubernetes_changed_by_projection: false,
  recovery_proven: false,
  observations: HISTORICAL_PROOF_PATHS.map((path) => ({ ...path })),
  limitations: [...HISTORICAL_PROOF_LIMITATIONS],
});

describe('G2 historical-only G1 display contract', () => {
  it('accepts the exact sanitized five-flow projection and one optional BOM', () => {
    const p = parseHistoricalLiveProofFile('\ufeff' + JSON.stringify(fixture()));
    expect(p.allowed_count).toBe(3);
    expect(p.denied_count).toBe(2);
    expect(p.current_enforcement_verified).toBe(false);
    expect(p.observations[3].id).toBe('edge-to-data');
    expect(p.observations[3].observation).toBe('denied');
  });
  it('rejects all extra private fields, unknown routes, and reordered or missing tests', () => {
    const f = fixture();
    expect(() => parseHistoricalLiveProof({ ...f, terraform_state_sha256: 'a'.repeat(64) })).toThrow();
    expect(() => parseHistoricalLiveProof({ ...f, kubeconfig: 'private-path' })).toThrow();
    expect(() => parseHistoricalLiveProof({ ...f, observations: f.observations.slice(0, 4) })).toThrow();
    const swapped = fixture();
    [swapped.observations[0], swapped.observations[1]] = [swapped.observations[1], swapped.observations[0]];
    expect(() => parseHistoricalLiveProof(swapped)).toThrow();
    const wrong = fixture();
    wrong.observations[4].observation = 'allowed';
    expect(() => parseHistoricalLiveProof(wrong)).toThrow();
  });
  it('refuses fabricated current validity, false counts, new labels, and amended limitations', () => {
    const f = fixture();
    for (const change of [
      { current_cluster_checked: true }, { current_enforcement_verified: true },
      { network_verified_at_capture: false }, { recovery_proven: true },
      { terraform_executed_by_projection: true }, { kubernetes_changed_by_projection: true },
      { denied_count: 1 }, { allowed_count: 4 },
      { evidence_status: 'live_verified_now' }, { source: 'untrusted' },
      { lab_label: 'Production' }, { schema_version: 'unexpected' },
      { limitations: ['All paths are universally secured'] },
    ]) expect(() => parseHistoricalLiveProof({ ...f, ...change })).toThrow();
  });
  it('rejects malformed times, oversized files, double BOM and trailing JSON', () => {
    for (const observed_utc of [
      'not-a-date', '2026-02-30T03:27:30Z', '2026-11-12T03:27:30-05:00',
      '2025-11-12T03:27:30Z', '2026-11-12T03:27:30.1234567890Z',
    ]) expect(() => parseHistoricalLiveProof({ ...fixture(), observed_utc })).toThrow();
    expect(() => parseHistoricalLiveProofFile(' '.repeat(33000))).toThrow();
    expect(() => parseHistoricalLiveProofFile('\ufeff\ufeff' + JSON.stringify(fixture()))).toThrow();
    expect(() => parseHistoricalLiveProofFile(JSON.stringify(fixture()) + ' null')).toThrow();
  });
});
