import { useEffect, useRef, useState } from 'react';
import { loadApprovedHistoricalContainmentMap } from '../api/client';
import { isHistoricalMapApproved } from '../api/containmentHistorical';
import type { HistoricalContainmentMap } from '../api/containmentHistorical';
import type { Capabilities } from '../api/contracts';
import Icon from './Icon';

type LoadState = 'idle' | 'loading' | 'ready' | 'error';
const titles: Record<string, string> = {
  'edge-to-app': 'Edge → App', 'app-to-edge': 'App → Edge',
  'app-to-data': 'App → Data', 'data-to-app': 'Data → App',
  'edge-to-data': 'Edge → Data', 'data-to-edge': 'Data → Edge',
};
const order = ['edge-to-app', 'app-to-edge', 'app-to-data', 'data-to-app', 'edge-to-data', 'data-to-edge'];

// A completely separate historical readout: it never overlays the declared
// diagram, updates planning counts, or infers a pass/fail policy verdict.
export function HistoricalMapReport({ map, onReload }: { map: HistoricalContainmentMap; onReload?: () => void }) {
  return <div className="cm-history-report" aria-label="Saved historical network observations">
    <div className="cm-history-summary"><div><span className="cm-eyebrow">Approved local file / Historical only</span>
      <h3>Observed reachability. <em>Not containment proof.</em></h3>
      <p>Saved v0.3 probe observations from the operator-confirmed <strong>{map.profile}</strong> profile. Their origin and causal meaning are not independently authenticated.</p></div>
      {onReload && <button type="button" className="button light-button" onClick={onReload}>Reload saved observations <Icon name="refresh" size={15}/></button>}
    </div>
    <div className="cm-history-metrics" aria-label="Historical observation counts">
      <div><span>Recorded directions</span><strong>{String(map.observed_count).padStart(2, '0')} / 06</strong><small>Historical connectivity observations only</small></div>
      <div><span>Unresolved directions</span><strong>{String(6 - map.observed_count).padStart(2, '0')}</strong><small>Unobserved and not assessed</small></div>
      <div><span>Enforcement verdict</span><strong>INCONCLUSIVE</strong><small>Not independently established</small></div>
    </div>
    <div className="cm-history-grid" role="list" aria-label="Saved six-route observations">
      {order.map((id, index) => {
        const route = map.routes.find((candidate) => candidate.id === id);
        if (!route) throw new Error('Historical route contract incomplete');
        return <article className="cm-history-route" role="listitem" key={id}>
          <div className="cm-history-route-heading"><span>R{String(index + 1).padStart(2, '0')}</span><h4>{titles[id]}</h4><span className="status-pill status-muted">{route.assessment === 'inconclusive' ? 'INCONCLUSIVE' : 'NOT ASSESSED'}</span></div>
          <dl><div><dt>Saved observation</dt><dd>{route.observed === 'not_observed' ? 'Not observed' : route.observed === 'reachable' ? 'Reachable' : 'Unreachable'}</dd></div>
            <div><dt>Recorded (UTC)</dt><dd>{route.captured_utc ? <time dateTime={route.captured_utc}>{route.captured_utc}</time> : 'No reliable capture'}</dd></div></dl>
        </article>;
      })}
    </div>
    <div className="cm-history-limitations"><h4>What these observations cannot prove</h4>
      <ul>{map.limitations.map((limitation) => <li key={limitation}>{limitation}</li>)}</ul>
      <p>Source: <code>{map.source}</code>. Neither a reachability match nor a mismatch is a verified security finding.</p>
    </div>
  </div>;
}

export default function HistoricalContainmentMapPanel({ capabilities, capabilityLoading }: {
  capabilities: Capabilities | null; capabilityLoading: boolean;
}) {
  const approved = isHistoricalMapApproved(capabilities);
  const [state, setState] = useState<LoadState>('idle');
  const [map, setMap] = useState<HistoricalContainmentMap | null>(null);
  const controller = useRef<AbortController | null>(null);

  useEffect(() => () => controller.current?.abort(), []);
  useEffect(() => {
    // Removing approval clears old historical data, even if the local API is
    // switched to ordinary mode while the user remains on this page.
    if (!approved) {
      controller.current?.abort();
      setMap(null);
      setState('idle');
    }
  }, [approved]);

  const request = () => {
    if (!approved) return;
    controller.current?.abort();
    const next = new AbortController();
    controller.current = next;
    setMap(null);
    setState('loading');
    void loadApprovedHistoricalContainmentMap(next.signal).then(
      (data) => { if (!next.signal.aborted) { setMap(data); setState('ready'); } },
      () => { if (!next.signal.aborted) { setMap(null); setState('error'); } },
    );
  };

  return <section className="cm-history" aria-label="Optional saved network observations">
    <div className="cm-history-intro"><span className="cm-eyebrow">05 / Separately approved historical evidence</span>
      <h2>Saved network observations.</h2>
      <p>Separate from the planning map above. Viewing these records does not run probes, contact Kubernetes, read Terraform state, or establish containment.</p></div>
    {capabilityLoading && <div className="cm-history-state" role="status">Checking the local server's historical access capability…</div>}
    {!capabilityLoading && !approved && <div className="cm-history-state"><Icon name="lock" size={22}/>
      <h3>Historical network observations locked</h3><p>The Go API has not advertised a separately approved v0.3 six-route record. No historical data is requested or synthesized in this mode.</p>
      <p className="cm-history-note">Approval is performed offline when starting the Go server, not from the browser.</p></div>}
    {!capabilityLoading && approved && state === 'idle' && <div className="cm-history-state"><Icon name="activity" size={22}/>
      <h3>Approved historical records available</h3><p>A previously reviewed local six-route suite was accepted by the Go process. Click to request the limited projection; its origin is not independently authenticated.</p>
      <button type="button" className="button button-dark" onClick={request}>Load saved observations <Icon name="arrow" size={15}/></button></div>}
    {!capabilityLoading && approved && state === 'loading' && <div className="cm-history-state" role="status">Loading the fixed historical projection from the local API…</div>}
    {!capabilityLoading && approved && state === 'error' && <div className="cm-history-state" role="alert"><Icon name="alert" size={22}/><h3>Historical observations unavailable</h3>
      <p>The request failed or the server returned an unapproved data shape. No historical observations were displayed.</p>
      <button type="button" className="button button-dark" onClick={request}>Retry saved observations <Icon name="refresh" size={15}/></button></div>}
    {!capabilityLoading && approved && state === 'ready' && map && <HistoricalMapReport map={map} onReload={request}/>}
  </section>;
}
