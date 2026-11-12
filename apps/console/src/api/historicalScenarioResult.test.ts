import { describe, expect, it } from 'vitest';
import {
  FD004_CLAIM, FD004_LIMITATIONS, FD004_REQUIREMENTS, SCENARIO_RESULT_SCHEMA,
  parseHistoricalScenarioResult, parseHistoricalScenarioResultFile,
} from './historicalScenarioResult';

export const h2Fixture = () => ({
  schema_version: SCENARIO_RESULT_SCHEMA, scenario_id: 'FD-004',
  scenario_claim: FD004_CLAIM, outcome: 'not_evaluated',
  evidence_relation: 'historical_observation_consistent_with_claim_at_capture',
  evidence_source: 'user_supplied_hash_checked_h1_review_unattested',
  observed_utc: '2026-11-12T03:27:30.2363946Z',
  supporting_routes: ['edge-to-data'],
  positive_controls: ['edge-to-app', 'app-to-data', 'data-to-edge-control'],
  mapped_controls_not_evaluated: ['C-004', 'C-005'],
  requirements: FD004_REQUIREMENTS.map((requirement) => ({ ...requirement })),
  scenario_evaluation_executed: false, contract_scope_verified: false,
  current_cluster_checked: false, original_receipt_authenticated: false,
  scenario_fully_certified: false, limitations: [...FD004_LIMITATIONS],
});

describe('H3 browser-only historical H2 scenario display', () => {
  it('accepts exactly the historical not-evaluated result', () => {
    const result = parseHistoricalScenarioResultFile('\ufeff' + JSON.stringify(h2Fixture()));
    expect(result.scenario_id).toBe('FD-004');
    expect(result.outcome).toBe('not_evaluated');
    expect(result.scenario_fully_certified).toBe(false);
    expect(result.supporting_routes).toEqual(['edge-to-data']);
    expect(result.requirements).toHaveLength(4);
  });
  it('rejects fabricated certification, execution, or scope', () => {
    const f = h2Fixture();
    for (const change of [
      { outcome: 'passed' }, { outcome: 'failed' }, { outcome: 'incomplete' },
      { scenario_evaluation_executed: true }, { current_cluster_checked: true },
      { contract_scope_verified: true }, { original_receipt_authenticated: true },
      { scenario_fully_certified: true }, { scenario_claim: 'New claim' },
      { schema_version: 'wrong' }, { scenario_id: 'FD-005' },
      { evidence_source: 'authenticated' }, { evidence_relation: 'policy_causation_proven' },
    ]) expect(() => parseHistoricalScenarioResult({ ...f, ...change })).toThrow();
  });
  it('rejects missing, reordered or altered evidence and certification gates', () => {
    const f = h2Fixture();
    for (const change of [
      { positive_controls: f.positive_controls.slice(0, 2) },
      { positive_controls: [...f.positive_controls].reverse() },
      { supporting_routes: ['data-to-app'] },
      { mapped_controls_not_evaluated: ['C-004'] },
      { requirements: f.requirements.slice(0, 3) },
      { requirements: f.requirements.map((x) => ({ ...x, status: 'met' })) },
      { limitations: ['Unbounded security assurance'] },
      { private_receipt_path: '/secret' },
      { kubeconfig: '/tmp' },
    ]) expect(() => parseHistoricalScenarioResult({ ...f, ...change })).toThrow();
  });
  it('rejects invalid timestamps, double BOM, trailing JSON, and oversized content', () => {
    const f = h2Fixture();
    for (const observed_utc of [
      'not-a-time', '2025-11-12T03:27:30Z', '2026-02-30T03:27:30Z',
      '2026-11-12T03:27:30-05:00', '2026-11-12T03:27:30.1234567890Z',
    ]) expect(() => parseHistoricalScenarioResult({ ...f, observed_utc })).toThrow();
    expect(() => parseHistoricalScenarioResultFile('\ufeff\ufeff' + JSON.stringify(f))).toThrow();
    expect(() => parseHistoricalScenarioResultFile(JSON.stringify(f) + ' null')).toThrow();
    expect(() => parseHistoricalScenarioResultFile(' '.repeat(33000))).toThrow();
  });
});
