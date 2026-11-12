// Public synthetic schema fixture only. Never import private F5/H1/H2 receipts.
import { describe, expect, it } from 'vitest';
import fixture from '../../../../tests/fixtures/fd004-h2-result.synthetic.json';
import { parseHistoricalScenarioResult, parseHistoricalScenarioResultFile } from './historicalScenarioResult';

describe('H4 post-merge Go <-> browser historical contract', () => {
  it('accepts the versioned synthetic Go H2 result, without a certified verdict', () => {
    const parsed = parseHistoricalScenarioResultFile(JSON.stringify(fixture));
    expect(parsed.outcome).toBe('not_evaluated');
    expect(parsed.scenario_id).toBe('FD-004');
    expect(parsed.supporting_routes).toEqual(['edge-to-data']);
    expect(parsed.positive_controls).toHaveLength(3);
    expect(parsed.requirements).toHaveLength(4);
    expect(parsed.current_cluster_checked).toBe(false);
    expect(parsed.scenario_fully_certified).toBe(false);
  });
  it('fails closed on privilege and evidence upgrades', () => {
    for (const change of [
      { outcome: 'passed' }, { outcome: 'incomplete' }, { outcome: 'failed' },
      { scenario_fully_certified: true }, { current_cluster_checked: true },
      { original_receipt_authenticated: true }, { scenario_evaluation_executed: true },
      { contract_scope_verified: true }, { positive_controls: ['edge-to-app'] },
      { unexpected_private_path: 'private.tfstate' },
    ]) expect(() => parseHistoricalScenarioResult({ ...fixture, ...change })).toThrow();
  });
});
