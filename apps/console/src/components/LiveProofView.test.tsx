import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import LiveProofView, { LiveProofReport } from './LiveProofView';
import { parseHistoricalLiveProof, HISTORICAL_LIVE_PROOF_SCHEMA, HISTORICAL_PROOF_PATHS, HISTORICAL_PROOF_LIMITATIONS } from '../api/historicalLiveProof';

const report = () => parseHistoricalLiveProof({
  schema_version: HISTORICAL_LIVE_PROOF_SCHEMA,
  source: 'explicit_private_f5_receipt_projection', evidence_status: 'historical_verified_at_capture_not_live',
  lab_label: 'Disposable KIND network-policy lab', observed_utc: '2026-11-12T03:27:30.2363946Z',
  allowed_count: 3, denied_count: 2, network_verified_at_capture: true,
  current_cluster_checked: false, current_enforcement_verified: false,
  terraform_executed_by_projection: false, kubernetes_changed_by_projection: false,
  recovery_proven: false, observations: HISTORICAL_PROOF_PATHS.map((p) => ({ ...p })),
  limitations: [...HISTORICAL_PROOF_LIMITATIONS],
});
describe('G2 evidence scope and user-selected JSON', () => {
  it('renders an empty view by default without suggesting that tests ran or data was auto-loaded', () => {
    const html = renderToStaticMarkup(<LiveProofView/>);
    expect(html).toContain('No verdict without a source');
    expect(html).toContain('NO FILE LOADED');
    expect(html).toContain('No cluster inspected');
    expect(html).toContain('Choose G1 sanitized historical proof JSON');
    expect(html).toContain('No local projection selected');
    expect(html).toContain('aria-live="polite"');
    expect(html).not.toContain('Loaded: ');
    expect(html).not.toContain('Captured, not current.');
    expect(html).not.toContain('NETWORK TRAFFIC ENFORCEMENT VERIFIED');
  });
  it('renders five verified-at-capture paths, with limits and no private identifiers', () => {
    const html = renderToStaticMarkup(<LiveProofReport proof={report()}/>);
    expect(html).toContain('Edge → App');
    expect(html).toContain('App → Data');
    expect(html).toContain('Data → Edge');
    expect(html).toContain('Edge → Data');
    expect(html).toContain('Data → App');
    expect(html).toContain('Observed allowed');
    expect(html).toContain('BLOCK');
    expect(html).toContain('Captured, not current.');
    expect(html).toContain('2026-11-12T03:27:30.2363946Z');
    expect(html).toContain('Five HTTP paths demonstrate bounded behavior');
    expect(html).toContain('font-size="14"');
    expect(html).not.toContain('terraform_state_sha256');
    expect(html).not.toContain('kubeconfig');
    expect(html).not.toContain('10.96.');
    expect((html.match(/class="lp-route /g) ?? [])).toHaveLength(5);
  });
});
