import { describe, expect, it, vi } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import App from '../App';
import { parsePlanningContainmentMap, type MapProfile, type MapRouteID } from '../api/containment';
import ContainmentMapView, { PlanningMapContent, nextPlanningRouteForKey } from './ContainmentMapView';

const routePairs = [
  ['edge-to-app', 'edge', 'app'], ['app-to-data', 'app', 'data'],
  ['edge-to-data', 'edge', 'data'], ['data-to-app', 'data', 'app'],
  ['app-to-edge', 'app', 'edge'], ['data-to-edge', 'data', 'edge'],
] as const;
const limitations = [
  'Zone nodes and routes are a validated fixed lab plan, not a current Kubernetes workload inventory.',
  'Probe expectations are planning-only; no Terraform plan receipt establishes declared policy intent.',
  'No network observations or enforcement checks were performed; every route remains not assessed.',
];
function map(profile: MapProfile) {
  return parsePlanningContainmentMap({
    schema_version: 'firedoor.v07.containment-map.v1',
    cluster: 'firedoor-local', context: 'kind-firedoor-local', profile,
    endpoint_mode: 'direct-pod-ip', transport: 'HTTP/TCP', port: 8080,
    source: 'declared_contract', current_inventory: false, enforcement_verified: false,
    nodes: ['edge', 'app', 'data'].map((id) => ({ id, kind: 'zone', zone: id, namespace: `firedoor-${id}` })),
    routes: routePairs.map(([id, from, to]) => {
      const expected = profile === 'hardened' && (id === 'edge-to-data' || id === 'data-to-app') ? 'unreachable' : 'reachable';
      return { id, from, to, declared: 'unspecified', observed: 'not_observed', assessment: 'not_assessed',
        reason: `The validated v0.3 plan expects ${expected} HTTP/TCP connectivity. This is planning-only, not a Terraform-backed declaration, measured result, or enforcement proof.` };
    }),
    evidence: [], limitations,
  }, profile);
}
function show(profile: MapProfile, route: MapRouteID = 'edge-to-app') {
  return renderToStaticMarkup(<PlanningMapContent map={map(profile)} selectedRoute={route} onSelectRoute={() => undefined}/>);
}

describe('7D1 visible read-only containment map', () => {
  it('adds a navigable Containment Map page without inventing a map on initial loading', () => {
    const spy = vi.spyOn(globalThis, 'fetch');
    try {
      const html = renderToStaticMarkup(<App/>);
      expect(html).toContain('Containment Map');
      expect(html).toContain('V0.7 DEV');
      const loading = renderToStaticMarkup(<ContainmentMapView/>);
      expect(loading).toContain('Loading the baseline planning map');
      expect(loading).toContain('Baseline');
      expect(loading).toContain('Hardened');
      expect(loading).not.toContain('class="cm-route"');
      expect(spy).not.toHaveBeenCalled();
    } finally { spy.mockRestore(); }
  });
  it('shows the six real planned directions, three zones, and no evidence or verdict', () => {
    const html = show('baseline');
    for (const zone of ['EDGE', 'APP', 'DATA']) expect(html).toContain(`>${zone}</text>`);
    for (const label of ['Edge → App', 'App → Edge', 'App → Data', 'Data → App', 'Edge → Data', 'Data → Edge']) expect(html).toContain(label);
    expect((html.match(/class="cm-route /g) ?? [])).toHaveLength(6);
    expect(html).toContain('aria-pressed="true"');
    expect(html).toContain('Not observed');
    expect(html).toContain('Not assessed');
    expect(html).toContain('Unspecified');
    expect(html).toContain('No observations loaded');
    expect(html).not.toContain('10.0.');
    expect(html).not.toContain('Policy enforcement verified');
    expect(html).not.toContain('COMPLIANT');
  });
  it('keeps baseline reachability strictly labeled as a planning expectation', () => {
    const html = show('baseline', 'edge-to-data');
    expect(html).toContain('Expected in v0.3 plan');
    expect(html).toContain('Reachable');
    expect(html).toContain('PLANNING ONLY');
    expect(html).toContain('not a Terraform-backed declaration');
    expect(html).toContain('aria-live="polite"');
  });
  it('shows hardened expected-unreachable only for the two reviewed routes, never as observed containment', () => {
    for (const id of ['edge-to-data', 'data-to-app'] as const) {
      const html = show('hardened', id);
      expect(html).toContain('Unreachable');
      expect(html).toContain('PLANNING ONLY');
      expect(html).toContain('Not observed');
      expect(html).toContain('Not assessed');
    }
    expect(show('hardened', 'edge-to-app')).toContain('Reachable');
  });
  it('supports predictable route arrow navigation and Home/End without inventing a route', () => {
    expect(nextPlanningRouteForKey('edge-to-app', 'ArrowRight')).toBe('app-to-edge');
    expect(nextPlanningRouteForKey('edge-to-app', 'ArrowLeft')).toBe('data-to-edge');
    expect(nextPlanningRouteForKey('app-to-data', 'ArrowDown')).toBe('data-to-app');
    expect(nextPlanningRouteForKey('data-to-edge', 'ArrowUp')).toBe('edge-to-data');
    expect(nextPlanningRouteForKey('data-to-app', 'Home')).toBe('edge-to-app');
    expect(nextPlanningRouteForKey('edge-to-app', 'End')).toBe('data-to-edge');
    expect(nextPlanningRouteForKey('app-to-data', 'Tab')).toBeNull();
    expect(nextPlanningRouteForKey('edge-to-data', 'Escape')).toBeNull();
  });
  it('links selectable route controls to an announced inspector and documents keyboard shortcuts', () => {
    const html = show('baseline', 'edge-to-data');
    expect(html).toContain('role="group"');
    expect((html.match(/aria-controls="cm-route-inspector"/g) ?? [])).toHaveLength(6);
    expect((html.match(/aria-describedby="cm-route-help"/g) ?? [])).toHaveLength(6);
    expect(html).toContain('id="cm-route-inspector"');
    expect(html).toContain('aria-live="polite"');
    expect(html).toContain('Arrow keys move to adjacent directions');
    expect(html).toContain('Selected direction');
    expect(html).toContain('Planning path highlight · no traffic measured');
  });
  it('makes expected profile differences explicit without implying observed enforcement', () => {
    const hardened = show('hardened', 'edge-to-data');
    expect(hardened).toContain('Plan expects unreachable');
    expect(hardened).toContain('hardened plan expects unreachable; not assessed');
    expect(hardened).toContain('NOT ASSESSED');
    expect(hardened).toContain('Not observed');
    const baseline = show('baseline', 'edge-to-data');
    expect(baseline).toContain('baseline plan expects reachable; not assessed');
    expect(baseline).not.toContain('hardened plan expects unreachable');
  });
  it('renders selectable route buttons, selected diagram highlight, and evidence limitations', () => {
    const html = show('baseline', 'data-to-edge');
    expect(html).toContain('aria-label="Select a planning route"');
    expect((html.match(/aria-pressed="true"/g) ?? [])).toHaveLength(1);
    expect((html.match(/aria-pressed="false"/g) ?? [])).toHaveLength(5);
    expect(html).toContain('data-to-edge');
    expect(html).toContain('Diagram geometry is illustrative');
    for (const limit of limitations) expect(html).toContain(limit);
    expect(html).not.toContain('docs/evidence/local/');
    expect(html).not.toContain('private_report');
  });
});
