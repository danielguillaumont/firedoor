import { useEffect, useRef, useState } from 'react';
import type { Capabilities, ControlCatalog, HistoricalEnvironment } from '../api/contracts';
import { loadControlCatalog, loadHistoricalEnvironment } from '../api/client';
import Icon from './Icon';

type Load<T> = { state: 'loading' | 'ready' | 'error'; data: T | null };

function SectionLabel({ children }: { children: string }) {
  return <div className="section-label"><span className="section-line"/>{children}</div>;
}

function ErrorPanel({ subject, onRetry }: { subject: string; onRetry: () => void }) {
  return <div role="alert" className="load-error"><Icon name="alert" size={20}/><div><strong>{subject} unavailable</strong>
    <p>The request failed or the response did not match Firedoor's strict read-only contract. No substitute values were displayed.</p></div>
    <button type="button" className="button text-button" onClick={onRetry}>Retry <Icon name="refresh" size={14}/></button></div>;
}

export function ControlCatalogPanel({ catalog }: { catalog: ControlCatalog }) {
  return <>
    <div className="control-stat"><div><span>GO ENGINE / FOUNDATION CATALOG</span><strong>{String(catalog.definitions.length).padStart(2, '0')} definitions</strong>
      <small>Specifications only. Zero control findings have been evaluated in this view.</small></div>
      <span className="status-pill status-muted">NOT EVALUATED</span></div>
    <section className="control-grid" aria-label="Go security control definitions">
      {catalog.definitions.map((entry) => <article key={entry.legacy_id} className="panel control-card">
        <div className="control-card-top"><span className="scenario-id">{entry.legacy_id}</span><span className="control-domain">{entry.domain}</span></div>
        <h2>{entry.name}</h2><p>{entry.description}</p>
        <div className="control-card-bottom"><span>Definition priority: <strong>{entry.severity}</strong></span><span className="status-pill status-muted">NOT EVALUATED</span></div>
      </article>)}
    </section>
    <p className="footnote">Source: <code>{catalog.source}</code>. No pass/fail result, control score, or Kubernetes inspection.</p>
    <section className="notice secondary"><Icon name="shield" size={19}/><div><strong>What this catalog cannot establish</strong>
      {catalog.limitations.map((limitation) => <span key={limitation}>{limitation}</span>)}</div></section>
  </>;
}

export function ControlsView({ refresh }: { refresh: number }) {
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<Load<ControlCatalog>>({ state: 'loading', data: null });
  useEffect(() => {
    const controller = new AbortController();
    setResult({ state: 'loading', data: null });
    void loadControlCatalog(controller.signal).then(
      (data) => { if (!controller.signal.aborted) setResult({ state: 'ready', data }); },
      () => { if (!controller.signal.aborted) setResult({ state: 'error', data: null }); },
    );
    return () => controller.abort();
  }, [refresh, attempt]);
  return <>
    <div className="title-row"><div><SectionLabel>03 / Control library · Specification desk</SectionLabel>
      <h1>Defined controls. <em>Not findings.</em></h1>
      <p className="lede">Six foundation control definitions from the Go engine. No active evaluation or security score is implied.</p></div></div>
    <div className="notice"><Icon name="info" size={19}/><div><strong>Read-only definitions</strong>
      <span>This page requests a static catalog, not cluster state. Severity describes a control's priority, not an observed vulnerability.</span></div></div>
    {result.state === 'loading' && <div className="panel panel-empty large" role="status">Loading six Go control definitions…</div>}
    {result.state === 'error' && <ErrorPanel subject="Control catalog" onRetry={() => setAttempt((n) => n + 1)}/>}
    {result.state === 'ready' && result.data && <ControlCatalogPanel catalog={result.data}/>}
  </>;
}

export function HistoricalEnvironmentPanel({ summary }: { summary: HistoricalEnvironment }) {
  return <>
    <div className="environment-verdict"><div><SectionLabel>Approved detached snapshot / historical only</SectionLabel>
      <h2>Snapshot. <em>Not a live inventory.</em></h2><p>Recorded Kubernetes configuration at collection time. Neither live cluster health nor policy enforcement was checked by this page.</p></div>
      <span className="status-pill status-muted">NOT CURRENT</span></div>
    <div className="proof-stat-grid environment-stats" aria-label="Historical environment counts">
      <div><span>Namespaces</span><strong>{summary.namespace_count}</strong><small>At historical capture</small></div>
      <div><span>Ready Pods</span><strong>{summary.ready_pod_count}</strong><small>Not current readiness</small></div>
      <div><span>Network policies</span><strong>{summary.policy_count}</strong><small>Presence does not prove enforcement</small></div>
    </div>
    <section className="panel environment-details"><SectionLabel>Observed environment / provenance</SectionLabel>
      <h2>Recorded lab context</h2>
      <dl><div><dt>Cluster</dt><dd>{summary.cluster}</dd></div><div><dt>Kubernetes context</dt><dd>{summary.context}</dd></div>
        <div><dt>Profile</dt><dd>{summary.profile}</dd></div><div><dt>Captured (UTC)</dt><dd>{summary.captured_utc}</dd></div>
        <div><dt>Live Kubernetes read</dt><dd>No</dd></div><div><dt>Enforcement verified</dt><dd>No</dd></div></dl>
    </section>
    <section className="panel environment-details"><SectionLabel>Evidence constraints</SectionLabel><h2>What this snapshot cannot prove</h2>
      <ul className="proof-limitations">{summary.limitations.map((limitation) => <li key={limitation}>{limitation}</li>)}</ul>
      <p className="footnote">Source: <code>{summary.source}</code>. No evidence files were loaded by the browser.</p></section>
  </>;
}

export function EnvironmentView({ capabilities, capabilityLoading }: { capabilities: Capabilities | null; capabilityLoading: boolean }) {
  const approved = capabilities?.mode === 'approved_detached_historical_report_read_only' &&
    capabilities.capabilities.some((entry) => entry.name === 'historical_environment_snapshot' && entry.availability === 'available' && entry.path === '/api/v1/environment-snapshot');
  const [state, setState] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle');
  const [summary, setSummary] = useState<HistoricalEnvironment | null>(null);
  const controller = useRef<AbortController | null>(null);
  useEffect(() => () => controller.current?.abort(), []);
  useEffect(() => { if (!approved) { controller.current?.abort(); setState('idle'); setSummary(null); } }, [approved]);
  const load = () => {
    if (!approved) return;
    controller.current?.abort();
    const next = new AbortController(); controller.current = next;
    setState('loading'); setSummary(null);
    void loadHistoricalEnvironment(next.signal).then(
      (data) => { if (!next.signal.aborted) { setSummary(data); setState('ready'); } },
      () => { if (!next.signal.aborted) { setSummary(null); setState('error'); } },
    );
  };
  return <>
    <div className="title-row"><div><SectionLabel>04 / Environment · Observation desk</SectionLabel>
      <h1>Know what was <em>observed.</em></h1>
      <p className="lede">A deliberately limited environment view. Approved historic configuration is not live inventory or containment proof.</p></div></div>
    {!approved && <section className="panel proof-locked"><div className="proof-icon"><Icon name="lock" size={26}/></div>
      <SectionLabel>Snapshot / not enabled</SectionLabel><h2>{capabilityLoading ? 'Checking API approval…' : 'Historical environment unavailable'}</h2>
      <p>The local API has not advertised an approved detached historical snapshot. This view will not contact Kubernetes, read files, or display placeholder metrics.</p>
      <div className="notice"><Icon name="info" size={19}/><div><strong>Server-side approval required</strong>
        <span>Start the Go API using the documented, explicit approved-proof procedure to enable the existing snapshot endpoint. This is not a live environment inventory.</span></div></div>
    </section>}
    {approved && state === 'idle' && <section className="panel proof-locked"><div className="proof-icon"><Icon name="server" size={26}/></div>
      <SectionLabel>Approved detached historical snapshot</SectionLabel><h2>Historical context is available</h2>
      <p>The Go API holds a previously approved historical summary in memory. Loading it is read-only and requires an explicit click here.</p>
      <button type="button" className="button button-dark" onClick={load}>Load historical snapshot <Icon name="arrow" size={16}/></button>
    </section>}
    {approved && state === 'loading' && <section className="panel proof-locked" role="status">Loading minimized historical environment snapshot…</section>}
    {approved && state === 'error' && <ErrorPanel subject="Historical environment" onRetry={load}/>}
    {approved && state === 'ready' && summary && <><div className="environment-reload"><button className="button light-button" type="button" onClick={load}>Reload historical snapshot <Icon name="refresh" size={15}/></button></div>
      <HistoricalEnvironmentPanel summary={summary}/></>}
  </>;
}
