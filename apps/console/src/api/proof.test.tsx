import { describe, expect, it, vi } from 'vitest';
import { parseCapabilities, parseProofSummary, PROOF_SCHEMA, SCHEMA } from './contracts';
import { endpoints, loadApprovedProofSummary } from './client';
import { renderToStaticMarkup } from 'react-dom/server';
import ProofLensView, { ProofLensReport } from '../components/ProofLensView';

const fixture = () => ({
  schema_version: PROOF_SCHEMA, source_report_schema: 'firedoor.v05.proof-report.v1',
  source_report_sha256: 'b'.repeat(64), verdict: 'inconclusive',
  evidence_status: 'historical_context_only_not_a_proof_chain',
  proof_chain_established: false, independently_attested: false, local_consistency_only: true,
  terraform: { receipt_id: 'TFPLAN-SYNTHETIC-TEST-00000001', captured_utc: '2026-11-05T17:22:35Z',
    source_sha256: 'a'.repeat(64), add: 0, change: 0, remove: 0, applied: false, evidence_type: 'speculative_plan_not_apply' },
  kubernetes: { cluster: 'firedoor-local', context: 'kind-firedoor-local', profile: 'hardened', captured_utc: '2026-11-05T17:25:35Z',
    namespace_count: 3, ready_pod_count: 3, policy_count: 2, evidence_type: 'read_only_snapshot_at_collection_time', current_read: false },
  historical_witnesses: ['data-to-app', 'edge-to-data'].map((route) => ({ route_id: route, captured_utc: '2026-11-04T17:25:35Z', historical_only: true,
    restoration_verified: true, phases: [ { phase: 'before', observed: 'unreachable' }, { phase: 'witness_present', observed: 'reachable' }, { phase: 'restored', observed: 'unreachable' } ] })),
  limitations: [
    'A speculative Terraform plan proposes changes; it does not apply infrastructure or establish Kubernetes enforcement.',
    'A fresh Kubernetes inventory is a configuration snapshot, not a live network traffic test.',
    'Historical v0.4 A/B/A HTTP observations predate this plan and are NOT contemporaneous or proof of a single causal chain.',
    'Restoration and journal checks are local record validations; no historical witness is replayed.',
    'SHA256 values only detect local inconsistency and are not signatures, remote attestations, or independently authenticated evidence.',
    'The overall Firedoor Proof Lens security verdict is inconclusive.',
  ],
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
    { name: 'control_catalog', availability: 'available', path: '/api/v1/control-catalog' },
    { name: 'historical_environment_snapshot', availability: 'available', path: '/api/v1/environment-snapshot' },
    { name: 'historical_activity', availability: 'available', path: '/api/v1/historical-activity' },
  ] });

describe('approved historical Proof Lens UI and API contracts', () => {
  it('accepts the exact server-advertised approved mode and fixed route', () => {
    const c = parseCapabilities(approvedCapabilities());
    expect(c.capabilities.filter((a) => a.availability === 'available')).toHaveLength(6);
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
    expect(report.verdict).toBe('inconclusive');
    expect(report.proof_chain_established).toBe(false);
    expect(report.historical_witnesses[0].phases[1].observed).toBe('reachable');
    expect(report.terraform.applied).toBe(false);
  });
  it('rejects fabricated verdicts, false apply, current reads, causal claims and altered history', () => {
    const changes: Array<(r: ReturnType<typeof fixture>) => void> = [
      (r) => { r.verdict = 'CONTAINED'; },
      (r) => { r.verdict = 'INCONCLUSIVE'; }, // Reject case drift: Go's canonical wire value is lowercase.
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
  it('rejects unknown raw-evidence fields, changed phase names and impossible chronology', () => {
    const withRaw = { ...fixture(), journal_sha256: 'a'.repeat(64) };
    expect(() => parseProofSummary(withRaw)).toThrow();
    const nested = clone(fixture()) as ReturnType<typeof fixture> & { kubernetes: ReturnType<typeof fixture>['kubernetes'] & { current_pod_secrets?: string } };
    nested.kubernetes.current_pod_secrets = 'never expose this';
    expect(() => parseProofSummary(nested)).toThrow();
    const misleading = clone(fixture());
    misleading.limitations[0] = 'All controls are secure';
    expect(() => parseProofSummary(misleading)).toThrow();
    const phases = clone(fixture());
    phases.historical_witnesses[0].phases[0].phase = 'A1';
    expect(() => parseProofSummary(phases)).toThrow();
    const chronology = clone(fixture());
    chronology.kubernetes.captured_utc = '2026-11-01T00:00:00Z';
    expect(() => parseProofSummary(chronology)).toThrow();
    const futureHistory = clone(fixture());
    futureHistory.historical_witnesses[0].captured_utc = '2026-11-06T00:00:00Z';
    expect(() => parseProofSummary(futureHistory)).toThrow();
  });
  it('shows only a deliberately clickable loading control without requesting the proof on initial render', () => {
    const spy = vi.spyOn(globalThis, 'fetch');
    try {
      const html = renderToStaticMarkup(<ProofLensView capabilities={parseCapabilities(approvedCapabilities())} />);
      expect(html).toContain('Load approved summary');
      expect(html).not.toContain('INCONCLUSIVE');
      expect(spy).not.toHaveBeenCalled();
    } finally { spy.mockRestore(); }
  });
  it('uses a bounded GET without browser credentials, and fails closed on unexpected payloads', async () => {
    const spy = vi.spyOn(globalThis, 'fetch');
    try {
      spy.mockResolvedValueOnce(new Response(JSON.stringify(fixture()), { headers: { 'Content-Type': 'application/json' } }));
      const summary = await loadApprovedProofSummary(new AbortController().signal);
      expect(summary.verdict).toBe('inconclusive');
      expect(spy).toHaveBeenCalledWith(endpoints.proof, expect.objectContaining({
        method: 'GET', credentials: 'omit', cache: 'no-store', redirect: 'error',
      }));
      spy.mockResolvedValueOnce(new Response(JSON.stringify({ ...fixture(), raw_evidence: 'leak' }), { headers: { 'Content-Type': 'application/json' } }));
      await expect(loadApprovedProofSummary(new AbortController().signal)).rejects.toThrow();
      spy.mockResolvedValueOnce(new Response(JSON.stringify({ ...fixture(), extra: 'x'.repeat(40 * 1024) }), { headers: { 'Content-Type': 'application/json' } }));
      await expect(loadApprovedProofSummary(new AbortController().signal)).rejects.toThrow();
    } finally { spy.mockRestore(); }
  });
});
