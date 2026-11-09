import { describe, expect, it, vi } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import HistoryView, { HistoricalTimelineReport } from './HistoryView';
import { HISTORY_LIMITATIONS, parseCapabilities, parseHistoricalActivity, SCHEMA } from '../api/contracts';

const activity = () => parseHistoricalActivity({
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
  limitations: [...HISTORY_LIMITATIONS],
});

const approved = () => parseCapabilities({
  schema_version: SCHEMA, mode: 'approved_detached_historical_report_read_only', limitations: ['Historical access has limits'],
  capabilities: [
    { name: 'health', availability: 'available', path: '/api/v1/health' },
    { name: 'scenario_contracts', availability: 'available', path: '/api/v1/scenarios' },
    { name: 'proof_reports', availability: 'available', path: '/api/v1/proof-summary' },
    { name: 'control_findings', availability: 'not_enabled' },
    { name: 'environment_inventory', availability: 'not_enabled' },
    { name: 'scenario_execution', availability: 'not_enabled' },
    { name: 'control_catalog', availability: 'available', path: '/api/v1/control-catalog' },
    { name: 'historical_environment_snapshot', availability: 'available', path: '/api/v1/environment-snapshot' },
    { name: 'historical_activity', availability: 'available', path: '/api/v1/historical-activity' },
    { name: 'historical_containment_map', availability: 'not_enabled' },
  ],
});

describe('6D3B historical activity UI safety', () => {
  it('starts locked without approved capability; no placeholder timeline', () => {
    const html = renderToStaticMarkup(<HistoryView capabilities={null} capabilityLoading={false}/>);
    expect(html).toContain('Historical activity unavailable');
    expect(html).toContain('No records are requested or fabricated');
    expect(html).not.toContain('history-timeline');
    expect(html).not.toContain('Speculative Terraform plan receipt');
  });
  it('offers a user-triggered load in approved mode, without automatically fetching', () => {
    const spy = vi.spyOn(globalThis, 'fetch');
    try {
      const html = renderToStaticMarkup(<HistoryView capabilities={approved()} capabilityLoading={false}/>);
      expect(html).toContain('Load historical activity');
      expect(html).not.toContain('history-timeline');
      expect(spy).not.toHaveBeenCalled();
    } finally { spy.mockRestore(); }
  });
  it('renders chronological artifacts and exact recorded UTC timestamps, not execution runs', () => {
    const html = renderToStaticMarkup(<HistoricalTimelineReport summary={activity()}/>);
    expect(html).toContain('Records, not');
    expect(html).toContain('INCONCLUSIVE');
    expect(html).toContain('PLAN NOT APPLIED');
    expect(html).toContain('NOT CURRENT');
    expect(html).toContain('NOT CAUSAL PROOF');
    expect(html).toContain('2026-11-06T00:18:01Z');
    expect(html).toContain('2026-11-05T06:56:50Z');
    expect(html.indexOf('Read-only Kubernetes observation')).toBeLessThan(html.indexOf('Speculative Terraform plan receipt'));
    expect(html.indexOf('Speculative Terraform plan receipt')).toBeLessThan(html.indexOf('data-to-app A/B/A historical observation'));
    expect((html.match(/history-event-content/g) ?? [])).toHaveLength(4);
    expect(html).toContain('Execution verified:');
    expect(html).not.toContain('Execution verified: Yes');
    expect(html).not.toContain('Enforcement confirmed');
  });
  it('explicitly shows unknown drift and why no comparison can be made', () => {
    const html = renderToStaticMarkup(<HistoricalTimelineReport summary={activity()}/>);
    expect(html).toContain('NOT');
    expect(html).toContain('ASSESSED');
    expect(html).toContain('Comparable snapshot pairs');
    expect(html).toContain('Drift detected');
    expect(html).toContain('Unknown');
    expect(html).toContain('Not assessed is not the same as no drift.');
    expect(html).toContain('Only one approved Kubernetes snapshot is available');
  });
  it('keeps the reload control optional and never prints raw local evidence', () => {
    const withoutReload = renderToStaticMarkup(<HistoricalTimelineReport summary={activity()}/>);
    expect(withoutReload).not.toContain('Reload history');
    const withReload = renderToStaticMarkup(<HistoricalTimelineReport summary={activity()} onReload={() => undefined}/>);
    expect(withReload).toContain('Reload history');
    expect(withReload).not.toContain('source_sha256');
    expect(withReload).not.toContain('docs/evidence/local');
    expect(withReload).not.toContain('receipt_id');
  });
});
