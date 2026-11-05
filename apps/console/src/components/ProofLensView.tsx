import { useEffect, useRef, useState } from 'react';
import type { Capabilities, ProofSummary } from '../api/contracts';
import { loadApprovedProofSummary } from '../api/client';
import Icon from './Icon';

type State = 'idle' | 'loading' | 'ready' | 'error';

// Requests the in-memory approved projection ONLY when the operator clicks.
// No file paths, consent values, or live operations are submitted by this UI.
export default function ProofLensView({ capabilities }: { capabilities: Capabilities | null }) {
  const approved = capabilities?.mode === 'approved_detached_historical_report_read_only' &&
    capabilities.capabilities.some((entry) => entry.name === 'proof_reports' && entry.availability === 'available' && entry.path === '/api/v1/proof-summary');
  const [state, setState] = useState<State>('idle');
  const [summary, setSummary] = useState<ProofSummary | null>(null);
  const controller = useRef<AbortController | null>(null);
  useEffect(() => () => controller.current?.abort(), []);
  const request = () => {
    if (!approved) return;
    controller.current?.abort();
    const next = new AbortController();
    controller.current = next;
    setSummary(null);
    setState('loading');
    void loadApprovedProofSummary(next.signal).then(
      (data) => { if (!next.signal.aborted) { setSummary(data); setState('ready'); } },
      () => { if (!next.signal.aborted) { setSummary(null); setState('error'); } },
    );
  };
  return <>
    <div className="title-row"><div><div className="section-label"><span className="section-line"/>02 / Proof Lens · Evidence desk</div>
      <h1>Follow the <em>evidence.</em></h1>
      <p className="lede">Declared intent. Recorded configuration. Historical behavior. Different sources, different limits.</p>
    </div></div>
    {!approved ? <section className="panel proof-locked"><div className="proof-icon"><Icon name="lock" size={27}/></div>
      <div className="section-label"><span className="section-line"/>Local report / Not enabled</div>
      <h2>No approved summary loaded</h2>
      <p>The API is serving planning contracts only. The Proof Lens view does not read local files, use demo data, or collect Kubernetes observations.</p>
      <div className="notice"><Icon name="info" size={18}/><div><strong>Approval happens outside the browser</strong><span>The Go API can load a previously generated, verified and explicitly approved JSON report at startup. The older Markdown report cannot be used as a substitute.</span></div></div>
    </section> : <>
      {state === 'idle' && <section className="panel proof-locked"><div className="proof-icon"><Icon name="eye" size={26}/></div>
        <div className="section-label"><span className="section-line"/>Approved detached report</div>
        <h2>Historical summary is available</h2><p>The Go API reports that a verified detached summary was approved at startup. Viewing it does not run Terraform, contact Kubernetes, or update the evidence.</p>
        <button type="button" className="button button-dark" onClick={request}>Load approved summary <Icon name="arrow" size={16}/></button>
      </section>}
      {state === 'loading' && <section className="panel proof-locked" role="status">Retrieving the approved, minimized historical summary…</section>}
      {state === 'error' && <section className="panel proof-locked" role="alert"><h2>Summary unavailable</h2><p>The request failed or the response did not match Firedoor's strict evidence contract. No fallback data was displayed.</p>
        <button type="button" className="button button-dark" onClick={request}>Retry approved summary <Icon name="refresh" size={16}/></button></section>}
      {state === 'ready' && summary && <ProofLensReport summary={summary} onReload={request}/>}
    </>}
  </>;
}

export function ProofLensReport({ summary, onReload }: { summary: ProofSummary; onReload?: () => void }) {
  return <div className="proof-report" aria-label="Approved historical Proof Lens summary">
    <div className="proof-verdict"><div><div className="section-label"><span className="section-line"/>Historical context / Not an attestation</div>
      <h2>INCONCLUSIVE</h2><p>Local record checks are consistent. Causal policy enforcement is not established.</p></div>
      {onReload && <button type="button" className="button light-button" onClick={onReload}>Reload <Icon name="refresh" size={15}/></button>}
    </div>
    <div className="proof-stat-grid">
      <div><span>Plan operations</span><strong>{summary.terraform.add} / {summary.terraform.change} / {summary.terraform.remove}</strong><small>Add / Change / Remove · speculative only</small></div>
      <div><span>Owned namespaces</span><strong>{summary.kubernetes.namespace_count}</strong><small>Snapshot at collection time</small></div>
      <div><span>Ready Pods / policies</span><strong>{summary.kubernetes.ready_pod_count} / {summary.kubernetes.policy_count}</strong><small>Read-only observations, not current health</small></div>
    </div>
    <div className="proof-columns">
      <section className="panel proof-section"><div className="section-label"><span className="section-line"/>01 / Terraform intent</div><h2>Planned, not applied.</h2>
        <p>The receipt describes a speculative plan. It is not an infrastructure change or a network-enforcement test.</p>
        <dl><div><dt>Receipt</dt><dd>{summary.terraform.receipt_id}</dd></div><div><dt>Captured (UTC)</dt><dd>{summary.terraform.captured_utc}</dd></div><div><dt>Source SHA-256</dt><dd className="proof-hash">{summary.terraform.source_sha256}</dd></div><div><dt>Applied</dt><dd>No</dd></div></dl>
      </section>
      <section className="panel proof-section"><div className="section-label"><span className="section-line"/>02 / Kubernetes observation</div><h2>Snapshot, not enforcement.</h2>
        <p>The recorded cluster inspection was read-only. Loading this page does not query Kubernetes.</p>
        <dl><div><dt>Lab</dt><dd>{summary.kubernetes.cluster}</dd></div><div><dt>Profile</dt><dd>{summary.kubernetes.profile}</dd></div><div><dt>Captured (UTC)</dt><dd>{summary.kubernetes.captured_utc}</dd></div><div><dt>Current read</dt><dd>No</dd></div></dl>
      </section>
    </div>
    <section className="panel proof-section"><div className="section-label"><span className="section-line"/>03 / Prior A/B/A observations</div><h2>Historical witnesses</h2>
      <p>These observations were recorded separately from the plan and snapshot. Restoration checks do not prove a single causal chain.</p>
      <div className="proof-witness-grid">{summary.historical_witnesses.map((w) => <div className="proof-witness" key={w.route_id}><strong>{w.route_id}</strong><span className="proof-timestamp">{w.captured_utc}</span>
        <div className="proof-phases">{w.phases.map((phase, i) => <div key={i}><small>{phase.phase}</small><span className={phase.observed === 'reachable' ? 'observed-reachable' : 'observed-unreachable'}>{phase.observed}</span></div>)}</div>
        <small>Historical only · Restoration checked</small></div>)}</div>
    </section>
    <section className="panel proof-section"><div className="section-label"><span className="section-line"/>04 / Evidence limitations</div><h2>What this does not prove</h2>
      <ul className="proof-limitations">{summary.limitations.map((item, i) => <li key={i}>{item}</li>)}</ul>
      <p className="proof-footer-note">Detached report fingerprint: <code>{summary.source_report_sha256}</code>. SHA-256 is an integrity check, not authentication or an attestation.</p>
    </section>
  </div>;
}
