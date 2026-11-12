import { useState } from 'react';
import type { ChangeEvent } from 'react';
import {
  MAX_SCENARIO_RESULT_BYTES, parseHistoricalScenarioResultFile,
} from '../api/historicalScenarioResult';
import type { HistoricalScenarioResult } from '../api/historicalScenarioResult';
import './ScenarioEvidenceReview.css';

// Browser-only H2 display. Does not fetch, persist, upload or authenticate source.
export default function ScenarioEvidenceReview({ onViewProof }: { onViewProof?: () => void }) {
  const [result, setResult] = useState<HistoricalScenarioResult | null>(null);
  const [loadedName, setLoadedName] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [fileKey, setFileKey] = useState(0);
  const clear = () => {
    setResult(null); setLoadedName(null); setError(null); setFileKey((n) => n + 1);
  };
  const onFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    clear();
    if (!file) return;
    if (file.size > MAX_SCENARIO_RESULT_BYTES || file.size === 0) {
      setError('H2 file is empty or exceeds the 32 KiB limit.');
      return;
    }
    try {
      const parsed = parseHistoricalScenarioResultFile(await file.text());
      setResult(parsed);
      setLoadedName(file.name.slice(0, 128));
    } catch {
      setError('That file does not match the strict H2 historical scenario-result contract.');
    }
  };
  return <section className="scenario-evidence-review" aria-label="Historical FD-004 evidence review">
    <div className="scenario-evidence-head">
      <span className="section-label">HISTORICAL / SUPPORTING EVIDENCE</span>
      <strong>Not a scenario verdict</strong>
    </div>
    <p>FD-004 remains <b>Not tested</b>. Select the sanitized H2 JSON generated on your laptop to inspect one supporting historical observation. This browser cannot independently authenticate its original receipt.</p>
    <label className="scenario-evidence-picker">
      <span>Select H2 scenario result (JSON)</span>
      <input key={fileKey} type="file" accept=".json,application/json" onChange={(event) => { void onFile(event); }}/>
    </label>
    {error && <p className="scenario-evidence-error" role="alert">{error}</p>}
    {result ? <>
      <div className="scenario-evidence-summary" role="status">
        <span>Supporting observation at capture</span>
        <strong>Edge → Data did not respond</strong>
        <small>Outcome: NOT EVALUATED · Current cluster checked: NO</small>
        <small>Captured UTC: {result.observed_utc}</small>
        <small>Selected file: {loadedName}</small>
      </div>
      <div className="scenario-evidence-subhead">Positive traffic controls</div>
      <p className="scenario-evidence-routes">Edge → App · App → Data · Data → Edge</p>
      <div className="scenario-evidence-subhead">Unmet or unverified certification gates</div>
      <ul className="scenario-evidence-gates">
        {result.requirements.map((gate) => <li key={gate.id}>
          <strong>{gate.id.replaceAll('_', ' ')} — {gate.status.replaceAll('_', ' ')}</strong>
          <span>{gate.detail}</span>
        </li>)}
      </ul>
      <div className="scenario-evidence-warning">Mapped controls C-004 and C-005 were not evaluated. This is not proof of current enforcement, policy causation, or production isolation.</div>
      <div className="scenario-evidence-actions">
        {onViewProof && <button type="button" className="button light-button" onClick={onViewProof}>Open Live Proof</button>}
        <button type="button" className="button light-button" onClick={clear}>Clear result</button>
      </div>
    </> : <p className="scenario-evidence-empty">No historical result loaded. The scenario is still Not tested.</p>}
  </section>;
}
