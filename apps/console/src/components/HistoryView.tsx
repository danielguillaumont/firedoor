import { useEffect, useRef, useState } from 'react';
import type { Capabilities, HistoricalActivity, HistoricalRecord } from '../api/contracts';
import { loadHistoricalActivity } from '../api/client';
import Icon from './Icon';

type State = 'idle' | 'loading' | 'ready' | 'error';

const category: Record<HistoricalRecord['kind'], { label: string; status: string; icon: 'git' | 'server' | 'activity'; className: string }> = {
  terraform_plan: { label: 'Terraform intent', status: 'PLAN NOT APPLIED', icon: 'git', className: 'history-plan' },
  kubernetes_snapshot: { label: 'Kubernetes observation', status: 'NOT CURRENT', icon: 'server', className: 'history-snapshot' },
  historical_witness: { label: 'Historical network witness', status: 'NOT CAUSAL PROOF', icon: 'activity', className: 'history-witness' },
};

function SectionLabel({ children }: { children: string }) {
  return <div className="section-label"><span className="section-line"/>{children}</div>;
}

// The data reaching this component must already have passed parseHistoricalActivity.
// No event is represented as a Firedoor run, successful deployment or verified enforcement.
export function HistoricalTimelineReport({ summary, onReload }: { summary: HistoricalActivity; onReload?: () => void }) {
  return <div className="history-report" aria-label="Approved historical activity report">
    <div className="history-verdict">
      <div><SectionLabel>Approved evidence / Historical record set</SectionLabel><h2>Records, not <em>runs.</em></h2>
        <p>Four independently captured historical artifacts. Overall security verdict: <strong>INCONCLUSIVE</strong>. No deployment or policy-enforcement result is established.</p></div>
      {onReload && <button type="button" className="button light-button" onClick={onReload}>Reload history <Icon name="refresh" size={15}/></button>}
    </div>
    <div className="history-metrics" aria-label="Historical activity summary">
      <div><span>Historical artifacts</span><strong>{String(summary.records.length).padStart(2, '0')}</strong><small>Different sources, not execution runs</small></div>
      <div><span>Verified executions</span><strong>00</strong><small>None established by these records</small></div>
      <div><span>Drift assessment</span><strong className="history-no-assessment">N/A</strong><small>No comparable observations</small></div>
    </div>
    <div className="history-layout">
      <section className="panel history-timeline-panel" aria-label="Chronological historical record timeline">
        <div className="history-section-heading"><div><SectionLabel>01 / Record chronology · Newest first</SectionLabel><h2>Historical activity</h2>
          <p>Capture times are recorded in UTC. Their ordering does not establish causation.</p></div><span className="status-pill status-muted">HISTORICAL ONLY</span></div>
        <ol className="history-timeline">
          {summary.records.map((record, i) => {
            const presentation = category[record.kind];
            return <li key={record.record_id} className={`history-event ${presentation.className}`}>
              <div className="history-event-marker" aria-hidden="true"><Icon name={presentation.icon} size={20}/></div>
              <article className="history-event-content">
                <div className="history-event-top"><span className="history-event-index">RECORD {String(i + 1).padStart(2, '0')} / {presentation.label}</span><span className="status-pill status-muted">{presentation.status}</span></div>
                <h3>{record.subject}</h3>
                <time dateTime={record.captured_utc}>{record.captured_utc}</time>
                <div className="history-event-bottom"><span>Source class: <code>{record.evidence_class}</code></span><span>Execution verified: <strong>No</strong></span></div>
              </article>
            </li>;
          })}
        </ol>
      </section>
      <aside className="history-side" aria-label="Drift and evidence interpretation">
        <section className="history-drift" aria-label="Drift assessment status">
          <div className="history-drift-icon"><Icon name="git" size={22}/></div>
          <SectionLabel>02 / Drift visibility</SectionLabel>
          <h2>NOT <em>ASSESSED.</em></h2>
          <p>Only one approved historical Kubernetes snapshot is available. Without comparable before/after observations, Firedoor cannot determine whether configuration drift occurred.</p>
          <dl><div><dt>Comparable snapshot pairs</dt><dd>{summary.drift.comparable_snapshots}</dd></div>
            <div><dt>Current inventory queried</dt><dd>No</dd></div><div><dt>Drift detected</dt><dd>Unknown</dd></div></dl>
          <p className="history-drift-warning">Not assessed is not the same as no drift.</p>
        </section>
        <section className="panel history-evidence-notes"><SectionLabel>03 / Interpretation boundaries</SectionLabel><h2>What this cannot prove</h2>
          <ul className="proof-limitations">{summary.limitations.map((line) => <li key={line}>{line}</li>)}</ul>
        </section>
      </aside>
    </div>
    <p className="footnote">Source: <code>{summary.source}</code>. Approved detached projection only. No raw evidence, current Kubernetes reads, Terraform operations or run execution occurred.</p>
  </div>;
}

export default function HistoryView({ capabilities, capabilityLoading }: { capabilities: Capabilities | null; capabilityLoading: boolean }) {
  const approved = capabilities?.mode === 'approved_detached_historical_report_read_only' &&
    capabilities.capabilities.some((entry) => entry.name === 'historical_activity' && entry.availability === 'available' && entry.path === '/api/v1/historical-activity');
  const [state, setState] = useState<State>('idle');
  const [summary, setSummary] = useState<HistoricalActivity | null>(null);
  const controller = useRef<AbortController | null>(null);
  useEffect(() => () => controller.current?.abort(), []);
  useEffect(() => {
    // Revoking approval also removes previously loaded history from the view.
    if (!approved) { controller.current?.abort(); setSummary(null); setState('idle'); }
  }, [approved]);

  const request = () => {
    if (!approved) return;
    controller.current?.abort();
    const next = new AbortController();
    controller.current = next;
    setState('loading'); setSummary(null);
    void loadHistoricalActivity(next.signal).then(
      (data) => { if (!next.signal.aborted) { setSummary(data); setState('ready'); } },
      () => { if (!next.signal.aborted) { setSummary(null); setState('error'); } },
    );
  };
  return <>
    <div className="title-row"><div><SectionLabel>05 / History · Evidence chronology</SectionLabel>
      <h1>A record of <em>what was captured.</em></h1>
      <p className="lede">A time-ordered view of separately captured observations, without confusing artifacts with runs or unknown drift with clean posture.</p></div></div>
    {!approved && <section className="panel proof-locked"><div className="proof-icon"><Icon name="lock" size={27}/></div>
      <SectionLabel>Historical activity / Not enabled</SectionLabel>
      <h2>{capabilityLoading ? 'Checking API approval…' : 'Historical activity unavailable'}</h2>
      <p>The local API has not advertised approved detached historical activity. No records are requested or fabricated in this state.</p>
      <div className="notice"><Icon name="info" size={19}/><div><strong>Approval is server-side</strong>
        <span>Load a previously vetted detached report using the documented local Go API approval procedure. This page cannot approve evidence or run a scan.</span></div></div>
    </section>}
    {approved && state === 'idle' && <section className="panel proof-locked"><div className="proof-icon"><Icon name="activity" size={27}/></div>
      <SectionLabel>Approved detached report / Explicit access</SectionLabel>
      <h2>Four historical records available</h2>
      <p>The approved Go API has a minimized historical activity projection in memory. Loading it shows recorded context only; it does not enumerate run history or collect new evidence.</p>
      <button type="button" className="button button-dark" onClick={request}>Load historical activity <Icon name="arrow" size={16}/></button>
    </section>}
    {approved && state === 'loading' && <section className="panel proof-locked" role="status">Loading approved historical records…</section>}
    {approved && state === 'error' && <section className="panel proof-locked" role="alert"><h2>Historical activity unavailable</h2>
      <p>The API request failed or the response failed strict validation. No substitute records were displayed.</p>
      <button type="button" className="button button-dark" onClick={request}>Retry historical activity <Icon name="refresh" size={16}/></button></section>}
    {approved && state === 'ready' && summary && <HistoricalTimelineReport summary={summary} onReload={request}/>}
  </>;
}
