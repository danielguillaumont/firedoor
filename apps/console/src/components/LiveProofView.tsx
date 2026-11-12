import { useState } from 'react';
import type { ChangeEvent, ReactNode } from 'react';
import { MAX_HISTORICAL_PROOF_BYTES, parseHistoricalLiveProofFile } from '../api/historicalLiveProof';
import type { HistoricalLiveProof, ProofPathID } from '../api/historicalLiveProof';
import Icon from './Icon';
import './LiveProofView.css';

const zoneMeta = {
  edge: { title: 'EDGE', code: 'ZONE 01', service: 'firedoor-edge' },
  app: { title: 'APP', code: 'ZONE 02', service: 'firedoor-app' },
  data: { title: 'DATA', code: 'ZONE 03', service: 'firedoor-data' },
} as const;
const positions: Record<ProofPathID, string> = {
  'edge-to-app': 'M 255 180 C 307 108 355 108 407 180',
  'app-to-data': 'M 573 180 C 628 108 679 108 732 180',
  'data-to-edge-control': 'M 811 256 C 815 369 171 369 175 256',
  'edge-to-data': 'M 175 163 C 175 49 812 49 812 163',
  'data-to-app': 'M 732 240 C 677 322 625 322 573 240',
};
const pathLabels: Record<ProofPathID, string> = {
  'edge-to-app': 'Edge → App', 'app-to-data': 'App → Data',
  'data-to-edge-control': 'Data → Edge', 'edge-to-data': 'Edge → Data',
  'data-to-app': 'Data → App',
};

export function LiveProofDiagram({ proof, selected }: { proof: HistoricalLiveProof | null; selected: ProofPathID }) {
  const paths = proof?.observations ?? [];
  return <div className="lp-map-scroll"><svg viewBox="0 0 980 410" className="lp-map"
    role="img" aria-label={proof ? 'Historical five-direction network map with three allowed and two blocked observations. Choose a route below to inspect its result.' : 'Three-zone network layout. No historical proof loaded.'}>
    <defs>
      <marker id="lp-green-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse"><path d="M1 1 L9 5 L1 9 Z" fill="#7cd5a3"/></marker>
      <marker id="lp-muted-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse"><path d="M1 1 L9 5 L1 9 Z" fill="#627c81"/></marker>
      <filter id="lp-glow"><feGaussianBlur stdDeviation="5"/></filter>
    </defs>
    <path d="M 0 25 H 980 M 0 390 H 980" stroke="#314e55" strokeWidth="1" strokeDasharray="2 9"/>
    {[90, 175, 260, 345].map((y) => <path key={y} d={`M 35 ${y} H 945`} stroke="#29444c" strokeWidth=".7" strokeDasharray="2 10" />)}
    {paths.map((path) => {
      const active = path.id === selected;
      const allow = path.observation === 'allowed';
      const stroke = allow ? '#7cd5a3' : '#ed9a78';
      return <g key={path.id}>
        {active && <path d={positions[path.id]} stroke={stroke} strokeWidth="10" opacity=".12" fill="none" filter="url(#lp-glow)"/>}
        <path d={positions[path.id]} stroke={active ? stroke : allow ? '#497e76' : '#886759'} strokeWidth={active ? 3.5 : 2}
          fill="none" strokeDasharray={allow ? undefined : '6 8'} opacity={active ? 1 : .68}
          markerEnd={allow ? (active ? 'url(#lp-green-arrow)' : 'url(#lp-muted-arrow)') : undefined}/>
        {!allow && <circle cx={path.id === 'edge-to-data' ? 500 : 650} cy={path.id === 'edge-to-data' ? 49 : 280} r={active ? 8 : 6} fill="#ed9a78" stroke="#162b32" strokeWidth="3"/>}
      </g>;
    })}
    {(['edge', 'app', 'data'] as const).map((zone, i) => {
      const x = [74, 392, 716][i];
      const details = zoneMeta[zone];
      return <g key={zone}>
        <rect x={x - 3} y="164" width="198" height="100" rx="7" fill="#0b1d24" opacity=".8"/>
        <rect x={x} y="160" width="192" height="94" rx="5" fill="#233e46" stroke="#6d8588" strokeWidth="1.5" />
        <rect x={x + 16} y="178" width="7" height="7" rx="1" fill={zone === 'data' ? '#e3b375' : '#7cd5a3'}/>
        <text x={x + 32} y="187" fill="#b7c7c3" fontSize="11" letterSpacing="1.8" fontWeight="700">{details.code}</text>
        <text x={x + 16} y="222" fill="#f8f5eb" fontSize="30" fontWeight="760">{details.title}</text>
        <text x={x + 16} y="241" fill="#a8bdbe" fontSize="11">{details.service}</text>
      </g>;
    })}
    <text x="30" y="30" fontSize="10" letterSpacing="2.3" fill="#89aaa9">FIREDOOR / 3-ZONE TESTBED</text>
    <text x="950" y="30" fontSize="10" textAnchor="end" letterSpacing="1.5" fill="#a8bdbe">HTTP / PORT 8080</text>
    <text x="490" y="401" textAnchor="middle" fontSize="11" fill="#a4b5b3">{proof ? 'Historical observed paths · illustrative layout, not a packet trace' : 'No evidence loaded · directional outcomes intentionally hidden'}</text>
  </svg></div>;
}

function Stat({ label, value, note, tone }: { label: string; value: string; note: string; tone: string }) {
  return <div className={`lp-stat lp-stat-${tone}`}><span>{label}</span><strong>{value}</strong><small>{note}</small></div>;
}
function KeyLabel({ children }: { children: ReactNode }) { return <span className="lp-eyebrow">{children}</span>; }

export function LiveProofReport({ proof }: { proof: HistoricalLiveProof }) {
  const [selected, setSelected] = useState<ProofPathID>('edge-to-app');
  const selectedPath = proof.observations.find((path) => path.id === selected) ?? proof.observations[0];
  const allowed = selectedPath.observation === 'allowed';
  return <>
    <div className="lp-stat-grid" aria-label="Historical evidence summary">
      <Stat label="Observed paths" value="05 / 05" tone="neutral" note="Five bounded HTTP tests" />
      <Stat label="Allowed at capture" value="03" tone="allow" note="Successful HTTP responses" />
      <Stat label="Blocked at capture" value="02" tone="deny" note="Connection timeouts" />
      <Stat label="Current live check" value="NO" tone="neutral" note="Historical capture only" />
    </div>
    <div className="lp-report-grid">
      <section className="lp-topology" aria-labelledby="lp-topology-title">
        <div className="lp-section-heading"><div><KeyLabel>01 / NETWORK OBSERVATIONS</KeyLabel><h2 id="lp-topology-title">The boundary, observed.</h2></div><div className="lp-legend"><span><i className="lp-dot lp-dot-allow"/>Allowed</span><span><i className="lp-dot lp-dot-deny"/>Blocked</span></div></div>
        <LiveProofDiagram proof={proof} selected={selected}/>
        <div className="lp-route-list" aria-label="Select a historical network route">
          {proof.observations.map((path, i) => <button type="button" key={path.id} onClick={() => setSelected(path.id)} aria-pressed={path.id === selected} aria-controls="lp-route-inspection" className={`lp-route ${path.id === selected ? 'lp-route-selected' : ''}`}>
            <span className="lp-route-index">{String(i + 1).padStart(2, '0')}</span><span className="lp-route-name">{pathLabels[path.id]}</span><span className={`lp-path-outcome ${path.observation}`}>{path.observation === 'allowed' ? 'ALLOW' : 'BLOCK'}</span>
          </button>)}
        </div>
      </section>
      <aside className="lp-aside">
        <section className="lp-detail" id="lp-route-inspection" aria-label="Selected historical path">
          <KeyLabel>02 / SELECTED OBSERVATION</KeyLabel>
          <h2>{pathLabels[selectedPath.id]}</h2>
          <span className={`lp-detail-status ${allowed ? 'allow' : 'deny'}`}><span className="lp-detail-symbol">{allowed ? '✓' : '×'}</span>{allowed ? 'Observed allowed' : 'Observed blocked'}</span>
          <dl><div><dt>Source</dt><dd>{zoneMeta[selectedPath.from].service}</dd></div><div><dt>Destination</dt><dd>{zoneMeta[selectedPath.to].service}</dd></div><div><dt>Transport</dt><dd>HTTP · TCP 8080</dd></div><div><dt>Method</dt><dd>In-pod BusyBox wget</dd></div><div><dt>Observation</dt><dd>{allowed ? 'Successful HTTP response' : 'Timed out / no HTTP response'}</dd></div><div><dt>Evidence class</dt><dd>Saved lab observation</dd></div></dl>
          <p className="lp-detail-note">This is a historical observation from a bounded local lab. Selecting another path does not execute a network request.</p>
        </section>
        <section className="lp-capture"><KeyLabel>03 / PROVENANCE</KeyLabel><h3>Captured, not current.</h3><p className="lp-time-label">Observed UTC</p><time dateTime={proof.observed_utc}>{proof.observed_utc}</time><div className="lp-capture-line"><Icon name="lock" size={16}/>The browser verified this display schema—not the original private receipt or today's cluster.</div></section>
      </aside>
    </div>
    <section className="lp-limitations"><KeyLabel>04 / EVIDENCE LIMITATIONS</KeyLabel><h2>What this proves—and what it doesn't.</h2><ul>{proof.limitations.map((item) => <li key={item}>{item}</li>)}</ul><p>The imported projection never authorizes or executes Terraform, Kubernetes, or new security tests.</p></section>
  </>;
}

export default function LiveProofView() {
  const [proof, setProof] = useState<HistoricalLiveProof | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const handleFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const selected = event.currentTarget.files?.[0];
    event.currentTarget.value = ''; // Same file can be selected again; never persist the path.
    if (!selected) return;
    setProof(null); setError(null);
    if (selected.size > MAX_HISTORICAL_PROOF_BYTES || selected.size === 0 || !selected.name.toLowerCase().endsWith('.json')) {
      setError('Choose a valid G1 JSON projection smaller than 32 KiB. No evidence was loaded.');
      return;
    }
    setLoading(true);
    try {
      const text = await selected.text();
      const checked = parseHistoricalLiveProofFile(text);
      setProof(checked);
    } catch {
      setError('The file does not match the strict G1 historical-proof display contract. Nothing was loaded.');
    } finally { setLoading(false); }
  };
  return <div className="lp-page">
    <div className="lp-hero">
      <div className="lp-hero-content"><KeyLabel>NETWORK EVIDENCE / HISTORICAL REPLAY</KeyLabel><h1>Security boundaries.<br/><em>Observed, not assumed.</em></h1><p>Five bounded traffic observations from a disposable KIND lab. Load a sanitized G1 projection to inspect the path-by-path outcome—without running a test.</p><div className="lp-hero-seals"><span><Icon name="shield" size={14}/> Historical capture</span><span><Icon name="lock" size={14}/> Browser-only file</span><span><Icon name="info" size={14}/> Not a live check</span></div></div>
      <div className="lp-hero-art" aria-hidden="true"><span className="lp-orbit lp-orbit-one"/><span className="lp-orbit lp-orbit-two"/><span className="lp-orbit lp-orbit-three"/><span className="lp-orbit-core">F / D</span></div>
    </div>
    <section className="lp-import" aria-label="Load sanitized historical proof"><div className="lp-import-info"><KeyLabel>00 / INPUT SOURCE</KeyLabel><h2>{proof ? 'Historical projection loaded' : 'Choose your verified projection.'}</h2><p>{proof ? 'Five observations are available below. You can replace or clear this local browser view.' : 'Select Firedoor-G1-SanitizedHistoricalProof.json from Downloads. No data is uploaded, stored, or read automatically.'}</p></div>
      <div className="lp-import-actions"><label htmlFor="lp-proof-file">Select G1 JSON file</label><input id="lp-proof-file" aria-label="Choose G1 sanitized historical proof JSON" type="file" accept=".json,application/json" onChange={(event) => { void handleFile(event); }} disabled={loading}/>{proof && <button type="button" className="lp-clear" onClick={() => { setProof(null); setError(null); }}>Clear evidence</button>}</div>
    </section>
    {error && <div className="lp-error" role="alert"><Icon name="alert" size={18}/><span>{error}</span></div>}
    {loading && <p className="lp-loading" role="status">Validating historical projection locally…</p>}
    {proof ? <LiveProofReport proof={proof}/> : <section className="lp-empty" aria-label="No historical evidence loaded"><div className="lp-empty-top"><KeyLabel>AWAITING USER-SUPPLIED EVIDENCE</KeyLabel><span className="lp-empty-stamp">NO FILE LOADED</span></div><h2>No verdict without a source.</h2><p>The topology below is illustrative. Outcomes remain hidden until a strict G1 JSON projection is explicitly selected.</p><LiveProofDiagram proof={null} selected="edge-to-app"/><div className="lp-empty-footer">No evidence loaded <span>·</span> No cluster inspected <span>·</span> No test executed</div></section>}
  </div>;
}
