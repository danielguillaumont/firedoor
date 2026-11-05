import { describe, expect, it } from 'vitest';
import { parseCapabilities, parseProofSummary, PROOF_SCHEMA, SCHEMA } from './contracts';
import { endpoints } from './client';
import { renderToStaticMarkup } from 'react-dom/server';
import { ProofLensReport } from '../components/ProofLensView';

const fixture = () => ({
  schema_version: PROOF_SCHEMA, source_report_schema: 'firedoor.v05.proof-report.v1',
  source_report_sha256: 'b'.repeat(64), verdict: 'INCONCLUSIVE',
  evidence_status: 'historical_context_only_not_a_proof_chain',
  proof_chain_established: false, independently_attested: false, local_consistency_only: true,
  terraform: { receipt_id: 'TFPLAN-20261105T172235Z-813ff79fe4d0fda4b35cac81', captured_utc: '2026-11-05T17:22:35Z',
    source_sha256: 'a'.repeat(64), add: 0, change: 0, remove: 0, applied: false, evidence_type: 'speculative_plan_not_apply' },
  kubernetes: { cluster: 'firedoor-local', context: 'kind-firedoor-local', profile: 'hardened', captured_utc: '2026-11-05T17:25:35Z',
    namespace_count: 3, ready_pod_count: 3, policy_count: 2, evidence_type: 'read_only_snapshot_at_collection_time', current_read: false },
  historical_witnesses: ['data-to-app', 'edge-to-data'].map((route) => ({ route_id: route, captured_utc: '2026-11-04T17:25:35Z', historical_only: true,
    restoration_verified: true, phases: [ { phase: 'A1', observed: 'unreachable' }, { phase: 'B', observed: 'reachable' }, { phase: 'A2', observed: 'unreachable' } ] })),
  limitations: [ 'Speculative plan, not apply', 'Read-only snapshot only', 'Historical results predate snapshot', 'Restoration checks are local', 'SHA-256 is not authentication', 'Overall result is inconclusive' ],
});
const clone = <T,>(data: T): T => JSON.parse(JSON.stringify(data)) as T;

const approvedCapabilities = () => ({ schema_version: SCHEMA, mode: 'approved_detached_historical_report_read_only',
  limitations: ['historical only'], capabilities: [
    { name: 'health', availability: 'available', path: '/api/v1/health' },
    { name: 'scenario_contracts', availability: 'available', path: '/api/v1/scenarios' },
    { name: 'proof_reports', availability: 'available', path: '/api/v1/proof-summary' },
    { name: 'control_findings', availability: 'not_enabled' },
    { name: 'environment_inventory', availability: 'not_enabled' },
    { name: 'scenario_execution', availability: 'not_enabled' },
  ] });

describe('approved historical Proof Lens UI and API contracts', () => {
  it('accepts the exact server-advertised approved mode and fixed route', () => {
    const c = parseCapabilities(approvedCapabilities());
    expect(c.capabilities.filter((a) => a.availability === 'available')).toHaveLength(3);
    expect(endpoints.proof).toBe('/api/v1/proof-summary');
  });
  it('rejects unapproved features and arbitrary endpoint paths', () => {
    const bad = approvedCapabilities();
    bad.capabilities[2].path = '/api/v1/terraform/apply';
    expect(() => parseCapabilities(bad)).toThrow();
    const more = approvedCapabilities();
    more.capabilities[3].availability = 'available';
    expect(() => parseCapabilities(more)).toThrow();
  });
  it('accepts and minimizes the verified, inconclusive 6C1 display contract', () => {
    const report = parseProofSummary(fixture());
    expect(report.verdict).toBe('INCONCLUSIVE');
    expect(report.proof_chain_established).toBe(false);
    expect(report.historical_witnesses[0].phases[1].observed).toBe('reachable');
    expect(report.terraform.applied).toBe(false);
  });
  it('rejects fabricated verdicts, false apply, current reads, causal claims and altered history', () => {
    const changes: Array<(r: ReturnType<typeof fixture>) => void> = [
      (r) => { r.verdict = 'CONTAINED'; },
      (r) => { r.proof_chain_established = true; },
      (r) => { r.independently_attested = true; },
      (r) => { r.terraform.applied = true; },
      (r) => { r.kubernetes.current_read = true; },
      (r) => { r.historical_witnesses[0].phases[1].observed = 'unreachable'; },
      (r) => { r.terraform.source_sha256 = 'not-a-hash'; },
    ];
    for (const change of changes) {
      const invalid = clone(fixture());
      change(invalid);
      expect(() => parseProofSummary(invalid)).toThrow();
    }
  });
  it('renders historical context without a fabricated enforcement success state', () => {
    const html = renderToStaticMarkup(<ProofLensReport summary={parseProofSummary(fixture())} />);
    expect(html).toContain('INCONCLUSIVE');
    expect(html).toContain('No');
    expect(html).toContain('Historical witnesses');
    expect(html).toContain('SHA-256 is an integrity check');
    expect(html).not.toContain('CONTAINED');
  });
});
