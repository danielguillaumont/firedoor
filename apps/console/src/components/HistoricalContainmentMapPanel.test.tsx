import { describe, expect, it, vi } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import HistoricalContainmentMapPanel, { HistoricalMapReport } from './HistoricalContainmentMapPanel';
import { parseHistoricalContainmentMap } from '../api/containmentHistorical';
import { historicalFixture } from '../api/containmentHistorical.fixtures';
import type { Capabilities } from '../api/contracts';
import ContainmentMapView from './ContainmentMapView';

const approved: Capabilities = { schema_version: 'firedoor.v06.console-api.v1', mode: 'approved_v07_historical_map_read_only',
  capabilities: [{ name: 'historical_containment_map', availability: 'available', path: '/api/v1/containment-map/historical' }], limitations: [] };

describe('7E2B independently gated historical readout', () => {
  it('keeps the Containment Map planning-only when no mode is approved', () => {
    const spy = vi.spyOn(globalThis, 'fetch');
    try {
      const html = renderToStaticMarkup(<ContainmentMapView/>);
      expect(html).toContain('Loading the baseline planning map');
      expect(html).toContain('Historical network observations locked');
      expect(html).not.toContain('Load saved observations');
      expect(spy).not.toHaveBeenCalled();
    } finally { spy.mockRestore(); }
  });
  it('does not fetch approved historical evidence automatically when rendering', () => {
    const spy = vi.spyOn(globalThis, 'fetch');
    try {
      const html = renderToStaticMarkup(<HistoricalContainmentMapPanel capabilities={approved} capabilityLoading={false}/>);
      expect(html).toContain('Load saved observations');
      expect(html).toContain('previously reviewed local six-route suite');
      expect(html).not.toContain('Observed reachability.');
      expect(spy).not.toHaveBeenCalled();
    } finally { spy.mockRestore(); }
  });
  it('keeps pending capability checks neutral, with no load button', () => {
    const html = renderToStaticMarkup(<HistoricalContainmentMapPanel capabilities={null} capabilityLoading/>);
    expect(html).toContain('Checking the local server');
    expect(html).not.toContain('Load saved observations');
  });
  it('renders only recorded historical measurements, unknowns and inconclusive outcomes', () => {
    const data = parseHistoricalContainmentMap(historicalFixture());
    const html = renderToStaticMarkup(<HistoricalMapReport map={data}/>);
    expect(html).toContain('Observed reachability.');
    expect(html).toContain('INCONCLUSIVE');
    expect(html).toContain('NOT ASSESSED');
    expect(html).toContain('Reachable');
    expect(html).toContain('Unreachable');
    expect(html).toContain('No reliable capture');
    expect(html).toContain('02 / 06');
    expect((html.match(/role="listitem"/g) ?? []).length).toBe(6);
    expect(html).not.toContain('C:/');
    expect(html).not.toContain('COMPLIANT');
    expect(html).not.toContain('enforcement verified');
  });
});
