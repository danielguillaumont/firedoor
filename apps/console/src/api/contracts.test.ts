import { describe, expect, it } from 'vitest';
import { parseCapabilities, parseHealth, parseScenarios, SCHEMA } from './contracts';

const health = { schema_version: SCHEMA, service: 'firedoor-console-api', status: 'ok', scope: 'local_http_process_only', cluster_checked: false, evidence_loaded: false };
const caps = { schema_version: SCHEMA, mode: 'offline_contracts_only', limitations: ['No cluster operations'], capabilities: [
  { name: 'health', availability: 'available', path: '/api/v1/health' },
  { name: 'scenario_contracts', availability: 'available', path: '/api/v1/scenarios' },
  { name: 'proof_reports', availability: 'not_enabled' },
  { name: 'control_findings', availability: 'not_enabled' },
  { name: 'environment_inventory', availability: 'not_enabled' },
  { name: 'scenario_execution', availability: 'not_enabled' },
  { name: 'control_catalog', availability: 'available', path: '/api/v1/control-catalog' },
  { name: 'historical_environment_snapshot', availability: 'not_enabled' },
] };
const names = ['Compromised Workload', 'Identity Overreach', 'Secret Boundary', 'Lateral Movement', 'Privileged Workload Admission'];
const scenarios = { schema_version: SCHEMA, source: 'versioned_planning_contracts_only', limitations: ['Planning only'], scenarios: names.map((name, i) => ({
  schema_version: '1.0.0', id: `FD-00${i + 1}`, name, claim: `${name} test not executed`, mapped_controls: ['C-004'],
  contract_status: 'spec_only_not_implemented', security_verdict: 'not_tested', evidence_loaded: false,
})) };
const clone = <T>(v: T): T => JSON.parse(JSON.stringify(v)) as T;

describe('fixed API v0.6 contract gates', () => {
  it('accepts only a process-health response that makes no cluster or evidence claim', () => {
    expect(parseHealth(health).cluster_checked).toBe(false);
    for (const [field, value] of [['cluster_checked', true], ['evidence_loaded', true], ['scope', 'cluster_health'], ['status', 'contained']] as const) {
      expect(() => parseHealth({ ...health, [field]: value })).toThrow();
    }
  });
  it('rejects unknown schema and missing response fields', () => {
    expect(() => parseHealth({ ...health, schema_version: 'future' })).toThrow();
    expect(() => parseHealth({ status: 'ok' })).toThrow();
  });
  it('parses eight explicit capability states and rejects unguarded proof access', () => {
    expect(parseCapabilities(caps).capabilities.filter((c) => c.availability === 'available')).toHaveLength(3);
    const altered = clone(caps);
    altered.capabilities[2].availability = 'available';
    expect(() => parseCapabilities(altered)).toThrow();
    const unknown = clone(caps);
    unknown.capabilities[2].name = 'terraform_apply';
    expect(() => parseCapabilities(unknown)).toThrow();
  });
  it('accepts exactly five untested planning contracts', () => {
    expect(parseScenarios(scenarios).scenarios).toHaveLength(5);
    const claimed = clone(scenarios);
    claimed.scenarios[3].security_verdict = 'contained';
    expect(() => parseScenarios(claimed)).toThrow();
    const loaded = clone(scenarios);
    loaded.scenarios[3].evidence_loaded = true;
    expect(() => parseScenarios(loaded)).toThrow();
  });
  it('refuses duplicate IDs and unknown contract IDs', () => {
    const duplicate = clone(scenarios);
    duplicate.scenarios[1].id = 'FD-001';
    expect(() => parseScenarios(duplicate)).toThrow();
    const unknown = clone(scenarios);
    unknown.scenarios[1].id = 'FD-999';
    expect(() => parseScenarios(unknown)).toThrow();
  });
  it('rejects malformed controls, unsupported source modes and incomplete scenario lists', () => {
    const invalid = clone(scenarios);
    invalid.scenarios[0].mapped_controls = ['../secret'];
    expect(() => parseScenarios(invalid)).toThrow();
    const source = clone(scenarios);
    source.source = 'live_kubernetes';
    expect(() => parseScenarios(source)).toThrow();
    const short = clone(scenarios);
    short.scenarios.pop();
    expect(() => parseScenarios(short)).toThrow();
  });
});

describe('6D1A control and historical environment capability boundaries', () => {
  it('advertises catalog definitions but not control findings or live inventory', () => {
    const result = parseCapabilities(caps);
    expect(result.capabilities.find((c) => c.name === 'control_catalog')?.path).toBe('/api/v1/control-catalog');
    expect(result.capabilities.find((c) => c.name === 'control_findings')?.availability).toBe('not_enabled');
    expect(result.capabilities.find((c) => c.name === 'historical_environment_snapshot')?.availability).toBe('not_enabled');
    const wrong = clone(caps);
    wrong.capabilities[6].path = '/api/v1/terraform/apply';
    expect(() => parseCapabilities(wrong)).toThrow();
  });
  it('refuses historical snapshot access without approval and refuses live inventory claims', () => {
    const enabled = clone(caps);
    enabled.capabilities[7].availability = 'available';
    enabled.capabilities[7].path = '/api/v1/environment-snapshot';
    expect(() => parseCapabilities(enabled)).toThrow();
    enabled.mode = 'approved_detached_historical_report_read_only';
    enabled.capabilities[2].availability = 'available';
    enabled.capabilities[2].path = '/api/v1/proof-summary';
    expect(parseCapabilities(enabled).capabilities.filter((c) => c.availability === 'available')).toHaveLength(5);
    enabled.capabilities[4].availability = 'available';
    expect(() => parseCapabilities(enabled)).toThrow();
  });
});
