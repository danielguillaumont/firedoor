import { useEffect, useState } from 'react';
import { loadPlanningContainmentMap } from '../api/client';
import { expectedPlanningReachability } from '../api/containment';
import type { MapProfile, MapRouteID, MapZone, PlanningContainmentMap } from '../api/containment';
import Icon from './Icon';

type MapLoad =
  | { state: 'loading'; data: null }
  | { state: 'error'; data: null }
  | { state: 'ready'; data: PlanningContainmentMap };

const routeOrder: readonly MapRouteID[] = [
  'edge-to-app', 'app-to-edge', 'app-to-data',
  'data-to-app', 'edge-to-data', 'data-to-edge',
];
const zoneLabels: Record<MapZone, string> = { edge: 'EDGE', app: 'APP', data: 'DATA' };
const routeLabels: Record<MapRouteID, string> = {
  'edge-to-app': 'Edge → App', 'app-to-edge': 'App → Edge',
  'app-to-data': 'App → Data', 'data-to-app': 'Data → App',
  'edge-to-data': 'Edge → Data', 'data-to-edge': 'Data → Edge',
};
// Line geometry is a fixed illustration of the six allowlisted zone pairs,
// not discovered network paths or currently running Pods.
const routePaths: Record<MapRouteID, string> = {
  'edge-to-app': 'M 244 172 C 310 137, 386 137, 452 172',
  'app-to-edge': 'M 452 218 C 386 253, 310 253, 244 218',
  'app-to-data': 'M 648 172 C 714 137, 790 137, 856 172',
  'data-to-app': 'M 856 218 C 790 253, 714 253, 648 218',
  'edge-to-data': 'M 169 151 C 169 60, 931 60, 931 151',
  'data-to-edge': 'M 931 239 C 931 329, 169 329, 169 239',
};

function ZoneNetworkDiagram({ selected }: { selected: MapRouteID }) {
  return <div className="cm-diagram-scroll">
    <svg className="cm-diagram" viewBox="0 0 1100 360" role="img"
      aria-label="Planning topology diagram with three zones: Edge, App, Data. Six directed routes. Use the route buttons below to inspect each direction.">
      <defs>
        <marker id="cm-arrow-muted" markerWidth="7" markerHeight="7" refX="6" refY="3.5" orient="auto" markerUnits="strokeWidth">
          <path d="M0 0 L7 3.5 L0 7 Z" fill="#739097" />
        </marker>
        <marker id="cm-arrow-selected" markerWidth="7" markerHeight="7" refX="6" refY="3.5" orient="auto" markerUnits="strokeWidth">
          <path d="M0 0 L7 3.5 L0 7 Z" fill="#efbe6c" />
        </marker>
      </defs>
      <path d="M 169 80 H 931" stroke="#435c64" strokeDasharray="3 9" strokeWidth="1" />
      {routeOrder.map((id) => <path key={id} d={routePaths[id]}
        fill="none" stroke={id === selected ? '#efbe6c' : '#739097'}
        strokeWidth={id === selected ? 4 : 2} strokeLinecap="round"
        strokeDasharray={id === selected ? undefined : '6 6'}
        markerEnd={`url(#cm-arrow-${id === selected ? 'selected' : 'muted'})`}
        opacity={id === selected ? 1 : 0.58} />)}
      {(['edge', 'app', 'data'] as const).map((zone, i) => {
        const x = [90, 460, 830][i];
        return <g key={zone}>
          <rect x={x} y="151" width="180" height="88" rx="5" fill="#324c55" stroke="#8ba5a7" strokeWidth="1.2" />
          <rect x={x + 14} y="167" width="7" height="7" fill="#e5ad55" />
          <text x={x + 30} y="176" fill="#d7e0dc" fontSize="10" fontWeight="700" letterSpacing="2">ZONE 0{i + 1}</text>
          <text x={x + 18} y="209" fill="#fff9ef" fontSize="25" fontWeight="740">{zoneLabels[zone]}</text>
          <text x={x + 18} y="226" fill="#aebfc1" fontSize="10">firedoor-{zone}</text>
        </g>;
      })}
      <text x="550" y="38" textAnchor="middle" fill="#aabcbf" letterSpacing="1.6" fontSize="11">DECLARED LAB ZONES / DIRECTIONAL ROUTES</text>
      <text x="550" y="352" textAnchor="middle" fill="#aabcbf" fontSize="11">Diagram geometry is illustrative · not a live packet path</text>
    </svg>
  </div>;
}

export function PlanningMapContent({ map, selectedRoute, onSelectRoute }: {
  map: PlanningContainmentMap; selectedRoute: MapRouteID; onSelectRoute: (id: MapRouteID) => void;
}) {
  const selected = map.routes.find((route) => route.id === selectedRoute);
  if (!selected) throw new Error('Selected route missing from validated planning map');
  const reachability = expectedPlanningReachability(map.profile, selected.id);
  return <>
    <div className="cm-counts" aria-label="Planning map summary">
      <div><span>Defined zones</span><strong>{map.nodes.length.toString().padStart(2, '0')}</strong><small>Lab planning scope only</small></div>
      <div><span>Directed routes</span><strong>{map.routes.length.toString().padStart(2, '0')}</strong><small>All awaiting assessment</small></div>
      <div><span>Observed routes</span><strong>00</strong><small>No observations loaded</small></div>
    </div>
    <div className="cm-layout">
      <section className="cm-graph-panel" aria-labelledby="cm-topology-heading">
        <div className="cm-graph-heading"><div><span className="cm-eyebrow">01 / DECLARED PLANNING TOPOLOGY</span><h2 id="cm-topology-heading">The six-route surface.</h2></div><span className="cm-muted-stamp">NOT ASSESSED</span></div>
        <ZoneNetworkDiagram selected={selectedRoute}/>
        <div className="cm-graph-caption"><Icon name="info" size={16}/><span>Selected direction is highlighted for inspection, not because traffic was observed. Every route remains <b>NOT OBSERVED</b>.</span></div>
        <div className="cm-route-header"><span className="cm-eyebrow">02 / ROUTE MATRIX</span><span>Select a direction for its planning contract</span></div>
        <div className="cm-route-grid" aria-label="Select a planning route">
          {routeOrder.map((id, index) => {
            const route = map.routes.find((candidate) => candidate.id === id);
            if (!route) throw new Error('Canonical route missing from validated map');
            return <button key={id} type="button" className={`cm-route ${id === selectedRoute ? 'cm-route-active' : ''}`}
              aria-pressed={id === selectedRoute} onClick={() => onSelectRoute(id)}>
              <span className="cm-route-number">R{String(index + 1).padStart(2, '0')}</span>
              <span className="cm-route-name">{routeLabels[id]}</span>
              <span className="cm-route-status">NOT ASSESSED</span>
            </button>;
          })}
        </div>
      </section>
      <aside className="cm-inspector" aria-label="Selected route details">
        <section className="cm-detail-card" aria-live="polite">
          <span className="cm-eyebrow">03 / SELECTED ROUTE</span>
          <h2>{routeLabels[selected.id]}</h2>
          <div className="cm-route-code">{selected.id}</div>
          <dl>
            <div><dt>Profile</dt><dd>{map.profile}</dd></div>
            <div><dt>Expected in v0.3 plan</dt><dd>{reachability === 'reachable' ? 'Reachable' : 'Unreachable'} <small>PLANNING ONLY</small></dd></div>
            <div><dt>Declared policy intent</dt><dd>Unspecified</dd></div>
            <div><dt>Network observation</dt><dd>Not observed</dd></div>
            <div><dt>Route assessment</dt><dd>Not assessed</dd></div>
            <div><dt>Protocol</dt><dd>{map.transport} / {map.port}</dd></div>
          </dl>
          <div className="cm-reason"><strong>Why this is not a finding</strong><p>{selected.reason}</p></div>
        </section>
        <section className="cm-boundaries"><span className="cm-eyebrow">04 / EVIDENCE BOUNDARIES</span><h2>What this view cannot prove</h2>
          <ul>{map.limitations.map((limitation) => <li key={limitation}>{limitation}</li>)}</ul>
          <p>Source: <code>{map.source}</code> · No live Kubernetes reads or Terraform changes.</p>
        </section>
      </aside>
    </div>
  </>;
}

export default function ContainmentMapView() {
  const [profile, setProfile] = useState<MapProfile>('baseline');
  const [selectedRoute, setSelectedRoute] = useState<MapRouteID>('edge-to-app');
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<MapLoad>({ state: 'loading', data: null });

  useEffect(() => {
    const controller = new AbortController();
    setResult({ state: 'loading', data: null });
    void loadPlanningContainmentMap(profile, controller.signal).then(
      (data) => { if (!controller.signal.aborted) setResult({ state: 'ready', data }); },
      () => { if (!controller.signal.aborted) setResult({ state: 'error', data: null }); },
    );
    return () => controller.abort();
  }, [profile, attempt]);

  // Do not show a response for the previous profile while a new request is pending.
  const current = result.state === 'ready' && result.data.profile === profile ? result.data : null;
  return <>
    <div className="title-row"><div><div className="section-label"><span className="section-line"/>06 / Containment Map · Planning desk</div>
      <h1>Follow the routes. <em>Question the proof.</em></h1>
      <p className="lede">Explore the lab's six intended probe directions, without confusing a connectivity plan with measured security enforcement.</p></div></div>
    <div className="cm-header"><div><span className="cm-eyebrow">FIREDOOR / V0.7 TOPOLOGY PREVIEW</span><h2>Map the boundary.<br/><em>Keep the evidence honest.</em></h2><p>Local, read-only planning contracts. The diagram does not display real Pods, firewall decisions, traffic flows or current cluster state.</p></div>
      <fieldset className="cm-profile-switch"><legend>Planning profile</legend>
        {(['baseline', 'hardened'] as const).map((value) => <button key={value} type="button" onClick={() => setProfile(value)} aria-pressed={profile === value} className={profile === value ? 'cm-profile-selected' : ''}>{value === 'baseline' ? 'Baseline' : 'Hardened'}</button>)}
      </fieldset></div>
    {current ? <PlanningMapContent map={current} selectedRoute={selectedRoute} onSelectRoute={setSelectedRoute}/> :
      result.state === 'error' ? <section className="panel cm-load-state" role="alert"><Icon name="alert" size={23}/><h2>Planning map unavailable</h2><p>The local map endpoint did not return an approved contract. No sample topology or fabricated findings will be displayed.</p><button type="button" className="button button-dark" onClick={() => setAttempt((n) => n + 1)}>Retry map request <Icon name="refresh" size={15}/></button></section> :
      <section className="panel cm-load-state" role="status">Loading the {profile} planning map from the localhost Go API…</section>}
    <p className="cm-page-note">Planning view only · 3 reviewed lab zones · 6 fixed directed routes · no observed findings · no enforcement claim.</p>
  </>;
}
