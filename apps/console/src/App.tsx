import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { loadCapabilities, loadHealth, loadScenarios } from './api/client';
import ProofLensView from './components/ProofLensView';
import { ControlsView, EnvironmentView } from './components/ObservationViews';
import HistoryView from './components/HistoryView';
import ContainmentMapView from './components/ContainmentMapView';
import ScenarioDetailsDialog from './components/ScenarioDetailsDialog';
import type { Capabilities, Health, Scenario, Scenarios } from './api/contracts';
import BrandMark from './components/BrandMark';
import Icon from './components/Icon';
import type { IconName } from './components/Icon';
import { initialDemoViewFromSearch } from './demoView';

type Page = 'overview' | 'scenarios' | 'capabilities' | 'proof' | 'controls' | 'environment' | 'history' | 'containment';
type Remote<T> = { state: 'loading' | 'ready' | 'error'; data: T | null; error: string | null };
const loading = <T,>(): Remote<T> => ({ state: 'loading', data: null, error: null });

const navItems: Array<{ page: Page | null; label: string; icon: IconName; note?: string }> = [
  { page: 'overview', label: 'Overview', icon: 'dashboard' },
  { page: 'scenarios', label: 'Scenarios', icon: 'layers' },
  { page: 'containment', label: 'Containment Map', icon: 'git' },
  { page: 'proof', label: 'Proof Lens', icon: 'eye' },
  { page: 'controls', label: 'Controls', icon: 'shield' },
  { page: 'environment', label: 'Environment', icon: 'server' },
  { page: 'history', label: 'History', icon: 'activity' },
];

function SectionLabel({ children }: { children: ReactNode }) {
  return <div className="section-label"><span className="section-line" />{children}</div>;
}

function ApiState({ health }: { health: Remote<Health> }) {
  const connected = health.state === 'ready';
  return <span className={`api-state ${connected ? 'online' : health.state === 'loading' ? 'pending' : 'offline'}`}>
    <span className="state-dot" />{connected ? 'Local API responding' : health.state === 'loading' ? 'Connecting to API' : 'Local API unavailable'}
  </span>;
}

export function ScenarioCard({ scenario, onOpen }: { scenario: Scenario; onOpen?: () => void }) {
  return <button className="scenario-card" type="button" onClick={onOpen} disabled={!onOpen}>
    <span className="scenario-card-top"><span className="scenario-id">{scenario.id}</span><span className="status-pill status-muted">Not tested</span></span>
    <span className="scenario-title">{scenario.name}</span>
    <span className="scenario-claim">{scenario.claim}</span>
    <span className="scenario-card-bottom"><span>{scenario.mapped_controls.join(' / ')}</span><span className="view-contract">View contract <Icon name="arrow" size={15}/></span></span>
  </button>;
}

function RetryError({ title, onRetry }: { title: string; onRetry: () => void }) {
  return <div className="load-error" role="alert">
    <Icon name="alert" size={20}/><div><strong>{title}</strong><p>Start <code>go run ./cmd/firedoor console-api serve</code>, then retry. No sample data will be substituted.</p></div>
    <button type="button" onClick={onRetry} className="button text-button">Retry <Icon name="refresh" size={14}/></button>
  </div>;
}

function MetricCard({ label, value, detail, tone }: { label: string; value: string; detail: string; tone?: string }) {
  return <div className="metric"><span className="metric-heading">{label}</span><strong className={tone ? `metric-value ${tone}` : 'metric-value'}>{value}</strong><span className="metric-detail">{detail}</span></div>;
}

function Overview({ health, scenarios, capabilities, retry, navigate }: { health: Remote<Health>; scenarios: Remote<Scenarios>; capabilities: Remote<Capabilities>; retry: () => void; navigate: (page: Page) => void }) {
  const scenarioCount = scenarios.data?.scenarios.length;
  const activeCount = capabilities.data?.capabilities.filter((c) => c.availability === 'available').length;
  return <>
    <div className="title-row"><div><SectionLabel>Overview / Inspection desk</SectionLabel><h1>Boundaries need <em>evidence.</em></h1><p className="lede">A local workspace for security intent, observed configuration, and the things we cannot yet prove.</p></div><div className="title-ornament" aria-hidden="true"><BrandMark size={118} light={false}/></div></div>
    <div className="notice"><Icon name="info" size={19}/><div><strong>Contract preview — not a security assessment</strong><span>Browser requests never run Terraform, Kubernetes, or SQLite. An approved historical summary is available only after explicit server-side approval and user action.</span></div></div>
    <section aria-label="Current API facts" className="metrics-grid">
      <MetricCard label="Local process" value={health.state === 'ready' ? 'ONLINE' : health.state === 'error' ? 'OFFLINE' : '—'} detail="API response, not cluster health" tone={health.state === 'ready' ? 'metric-online' : ''}/>
      <MetricCard label="Scenario contracts" value={scenarioCount === undefined ? '—' : String(scenarioCount).padStart(2, '0')} detail="Planning contracts, not executed runs"/>
      <MetricCard label="Available API capabilities" value={activeCount === undefined ? '—' : String(activeCount).padStart(2, '0')} detail="Read-only contracts; historical routes require approval"/>
    </section>
    {(health.state === 'error' || scenarios.state === 'error' || capabilities.state === 'error') && <RetryError title="Some local API data could not be loaded" onRetry={retry}/>}
    <div className="overview-grid">
      <section className="panel scenario-panel"><div className="panel-head"><div><SectionLabel>01 / Validation catalogue</SectionLabel><h2>Scenario contracts</h2><p>Five defined security questions, awaiting full scenario certification.</p></div><button type="button" className="button light-button" onClick={() => navigate('scenarios')}>View all <Icon name="arrow" size={15}/></button></div>
        {scenarios.data ? <div className="compact-scenarios">{scenarios.data.scenarios.slice(0, 3).map((s) => <div className="compact-scenario" key={s.id}><div className="compact-id">{s.id}</div><div className="compact-copy"><strong>{s.name}</strong><small>{s.mapped_controls.join(' · ')}</small></div><span className="status-pill status-muted">Not tested</span></div>)}</div> : <div className="panel-empty">{scenarios.state === 'loading' ? 'Loading scenario contracts…' : 'No scenario contracts available.'}</div>}
      </section>
      <section className="panel evidence-panel"><div className="evidence-top"><div className="evidence-symbol"><Icon name="eye" size={21}/></div><span className="eyebrow">{capabilities.data?.mode === 'approved_detached_historical_report_read_only' ? 'APPROVED HISTORICAL VIEW' : 'MODULE / LOCKED'}</span></div><SectionLabel>02 / Proof Lens</SectionLabel><h2>Proof over posture.</h2><p>The CLI supports guarded evidence reporting. A previously approved historical summary can be viewed here without running collection.</p><div className="future-bar"><span>API access</span><strong>{capabilities.data?.mode === 'approved_detached_historical_report_read_only' ? 'Approved' : 'Not enabled'}</strong></div><div className="future-bar"><span>Enforcement claim</span><strong>Not established</strong></div><button type="button" className="coming-soon proof-open" onClick={() => navigate('proof')}>OPEN / PROOF LENS <Icon name="arrow" size={15}/></button></section>
    </div>
    <section className="principle"><div><SectionLabel>Inspection principle</SectionLabel><h2>Declared. Observed. Tested.</h2><p>These are separate evidence layers. Correlation is useful; causation requires stronger controls.</p></div><div className="principle-stages"><span>01 <b>Terraform intent</b></span><Icon name="arrow"/><span>02 <b>Kubernetes observation</b></span><Icon name="arrow"/><span>03 <b>Bounded behavior</b></span></div></section>
  </>;
}

function ScenarioView({ data, retry }: { data: Remote<Scenarios>; retry: () => void }) {
  const [selected, setSelected] = useState<string | null>(null);
  const record = data.data?.scenarios.find((s) => s.id === selected);
  return <><div className="title-row"><div><SectionLabel>Security library / FD series</SectionLabel><h1>Five questions. <em>No shortcuts.</em></h1><p className="lede">Versioned scenario contracts from the Go API. No execution or verification results are loaded in this slice.</p></div></div>
    <div className="notice"><Icon name="info" size={19}/><div><strong>Scenario status: not tested</strong><span>Historical network experiments exist separately; they do not certify any of these five contracts.</span></div></div>
    {data.state === 'error' && <RetryError title="Scenario catalogue unavailable" onRetry={retry}/>}
    {data.data ? <div className="scenario-grid">{data.data.scenarios.map((s) => <ScenarioCard scenario={s} key={s.id} onOpen={() => setSelected(s.id)}/>)}</div> : <div className="panel-empty large">{data.state === 'loading' ? 'Loading contracts from localhost…' : 'No scenario data available.'}</div>}
    <p className="footnote">Source: <code>versioned_planning_contracts_only</code>. No Kubernetes request was made.</p>
    {record && <ScenarioDetailsDialog scenario={record} onClose={() => setSelected(null)}/>}
  </>;
}

const capabilityLabels: Record<string, string> = {
  health: 'API process health', scenario_contracts: 'Scenario planning contracts', proof_reports: 'Proof Lens reports',
  control_findings: 'Control findings', environment_inventory: 'Environment inventory', scenario_execution: 'Scenario execution',
  control_catalog: 'Control definition catalog (not evaluated)', historical_environment_snapshot: 'Approved historical environment snapshot', historical_activity: 'Approved historical activity (not execution runs)', historical_containment_map: 'Saved v0.3 network observations (historical only)',
};

function CapabilitiesView({ data, retry }: { data: Remote<Capabilities>; retry: () => void }) {
  return <><div className="title-row"><div><SectionLabel>System / API contract</SectionLabel><h1>Capability <em>register.</em></h1><p className="lede">A precise record of what the localhost API currently makes available, and what remains disabled.</p></div></div>
    {data.state === 'error' && <RetryError title="Capability register unavailable" onRetry={retry}/>}
    <section className="panel capability-panel"><div className="panel-head"><div><SectionLabel>Runtime feature flags</SectionLabel><h2>Current availability</h2></div><span className="status-pill status-muted">{data.data?.mode === 'approved_detached_historical_report_read_only' ? 'Approved historical report' : data.data?.mode === 'approved_v07_historical_map_read_only' ? 'Approved historical map' : 'Offline contracts only'}</span></div>
      {data.data ? data.data.capabilities.map((item) => <div className="capability-row" key={item.name}><div className={`cap-icon ${item.availability === 'available' ? 'cap-icon-on' : ''}`}><Icon name={item.availability === 'available' ? 'check' : 'lock'} size={18}/></div><div><strong>{capabilityLabels[item.name] ?? item.name}</strong><small>{item.path ?? 'No HTTP route exposed'}</small></div><span className={`status-pill ${item.availability === 'available' ? 'status-available' : 'status-muted'}`}>{item.availability === 'available' ? 'Available' : 'Not enabled'}</span></div>) : <div className="panel-empty">{data.state === 'loading' ? 'Reading capability contract…' : 'No capability data available.'}</div>}
    </section><div className="notice secondary"><Icon name="shield" size={19}/><div><strong>Runtime boundary</strong><span>Actions cannot be enabled by changing this UI. Terraform and Kubernetes mutations remain outside the console API.</span></div></div>
  </>;
}

export default function App() {
  const [page, setPage] = useState<Page>(() =>
    typeof window !== 'undefined' ? initialDemoViewFromSearch(window.location.search) : 'overview');
  const [refresh, setRefresh] = useState(0);
  const [health, setHealth] = useState<Remote<Health>>(() => loading<Health>());
  const [capabilities, setCapabilities] = useState<Remote<Capabilities>>(() => loading<Capabilities>());
  const [scenarios, setScenarios] = useState<Remote<Scenarios>>(() => loading<Scenarios>());
  const retry = () => setRefresh((n) => n + 1);

  useEffect(() => {
    const controller = new AbortController();
    setHealth(loading<Health>()); setCapabilities(loading<Capabilities>()); setScenarios(loading<Scenarios>());
    void loadHealth(controller.signal).then((data) => { if (!controller.signal.aborted) setHealth({ state: 'ready', data, error: null }); }, (error: unknown) => { if (!controller.signal.aborted) setHealth({ state: 'error', data: null, error: String(error) }); });
    void loadCapabilities(controller.signal).then((data) => { if (!controller.signal.aborted) setCapabilities({ state: 'ready', data, error: null }); }, (error: unknown) => { if (!controller.signal.aborted) setCapabilities({ state: 'error', data: null, error: String(error) }); });
    void loadScenarios(controller.signal).then((data) => { if (!controller.signal.aborted) setScenarios({ state: 'ready', data, error: null }); }, (error: unknown) => { if (!controller.signal.aborted) setScenarios({ state: 'error', data: null, error: String(error) }); });
    return () => controller.abort();
  }, [refresh]);

  return <><a className="skip-link" href="#firedoor-main">Skip to main content</a><div className="shell"><aside className="sidebar"><div className="brand"><BrandMark size={43}/><div className="brand-text"><strong>FIREDOOR</strong><small>CONTAIN THE SPREAD.</small></div></div>
    <div className="workspace-label">WORKSPACE <span>V0.7 DEV</span></div>
    <nav aria-label="Main navigation" className="nav">{navItems.map((item) => <button key={item.label} type="button" disabled={item.page === null} onClick={() => item.page && setPage(item.page)} className={`nav-item ${page === item.page ? 'active' : ''}`} aria-current={page === item.page ? 'page' : undefined}><Icon name={item.icon} size={19}/><span>{item.label}</span>{item.note && <small>{item.note}</small>}</button>)}</nav>
    <div className="sidebar-bottom"><div className="sidebar-cap"><span className="tiny-square"/> DEVELOPMENT POSTURE</div><div className="sidebar-desc">No live validations, Terraform operations, or automatic evidence reads.</div><button type="button" className="sidebar-system" onClick={() => setPage('capabilities')}><Icon name="activity" size={17}/> API capabilities <Icon name="chevron" size={17}/></button></div>
  </aside><main id="firedoor-main" tabIndex={-1} className="main"><header className="topbar"><div className="breadcrumb">WORKSPACE <span>/</span> <strong>{page === 'overview' ? 'OVERVIEW' : page === 'scenarios' ? 'SCENARIOS' : page === 'containment' ? 'CONTAINMENT MAP' : page === 'proof' ? 'PROOF LENS' : page === 'controls' ? 'CONTROLS' : page === 'environment' ? 'ENVIRONMENT' : page === 'history' ? 'HISTORY' : 'CAPABILITIES'}</strong></div><div className="topbar-actions"><ApiState health={health}/><button className="refresh-button" type="button" onClick={retry} aria-label="Refresh local API responses" title="Refresh local API responses"><Icon name="refresh" size={17}/></button></div></header><div className="content" key={page}>{page === 'overview' ? <Overview health={health} capabilities={capabilities} scenarios={scenarios} retry={retry} navigate={setPage}/> : page === 'scenarios' ? <ScenarioView data={scenarios} retry={retry}/> : page === 'containment' ? <ContainmentMapView key={`${refresh}-${capabilities.state}-${capabilities.data?.mode ?? 'unavailable'}`} capabilities={capabilities.data} capabilityLoading={capabilities.state === 'loading'}/> : page === 'proof' ? <ProofLensView key={`${refresh}-${capabilities.state}-${capabilities.data?.mode ?? 'unavailable'}`} capabilities={capabilities.data}/> : page === 'controls' ? <ControlsView refresh={refresh}/> : page === 'environment' ? <EnvironmentView capabilities={capabilities.data} capabilityLoading={capabilities.state === 'loading'}/> : page === 'history' ? <HistoryView key={`${refresh}-${capabilities.state}-${capabilities.data?.mode ?? 'unavailable'}`} capabilities={capabilities.data} capabilityLoading={capabilities.state === 'loading'}/> : <CapabilitiesView data={capabilities} retry={retry}/>}</div><footer className="footer"><span>FIREDOOR / LOCAL INSPECTION DESK</span><span>GO ENGINE <b>·</b> TERRAFORM INTENT <b>·</b> KUBERNETES EVIDENCE</span><span>V0.7 DEV / PLANNING MAP, NOT ENFORCEMENT</span></footer></main></div></>;
}
