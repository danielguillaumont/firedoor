import { describe, expect, it, vi } from 'vitest';
import type { UserConfig } from 'vite';
import viteConfig from '../../vite.config';
import { renderToStaticMarkup } from 'react-dom/server';
import { endpoints, loadControlCatalog, loadHistoricalEnvironment } from './client';
import { parseCapabilities, parseControlCatalog, parseHistoricalEnvironment, SCHEMA } from './contracts';
import { ControlCatalogPanel, ControlsView, EnvironmentView, HistoricalEnvironmentPanel } from '../components/ObservationViews';

const catalogFixture = () => ({
  schema_version: SCHEMA, source: 'go_control_definitions_only', evaluation_status: 'not_evaluated',
  definitions: [
    ['CTRL-ID-001', 'C-001', 'Dedicated workload ServiceAccount', 'identity', 'medium'],
    ['CTRL-ID-002', 'C-002', 'Disable unnecessary token automount', 'identity', 'high'],
    ['CTRL-ID-003', 'C-003', 'Least-privilege Kubernetes RBAC', 'identity', 'high'],
    ['CTRL-NET-001', 'C-004', 'Default-deny network policy', 'network', 'high'],
    ['CTRL-WL-001', 'C-006', 'Run as non-root', 'workload', 'high'],
    ['CTRL-WL-002', 'C-007', 'Disable privilege escalation', 'workload', 'high'],
  ].map(([id, legacy_id, name, domain, severity]) => ({ id, legacy_id, name, domain, severity, description: 'Control specification — not a finding.' })),
  limitations: [
    'Control definitions are specifications, not observed findings or enforcement proof.',
    'No Kubernetes inspection, Terraform inspection, or control scoring occurred in this HTTP request.',
  ],
});
const environmentFixture = () => ({
  schema_version: SCHEMA, source: 'approved_detached_historical_report_only',
  observation_status: 'historical_snapshot_not_current', cluster: 'firedoor-local', context: 'kind-firedoor-local', profile: 'hardened',
  captured_utc: '2026-11-05T17:23:30Z', namespace_count: 3, ready_pod_count: 3, policy_count: 2,
  current_read: false, enforcement_verified: false,
  limitations: [
    'This is an approved historical Kubernetes snapshot, not a current cluster inventory.',
    'Policy counts and ready Pod counts do not establish enforcement or network containment.',
    'The original detached report is not an independently authenticated attestation.',
  ],
});
const capabilitiesFixture = (approved: boolean) => parseCapabilities({
  schema_version: SCHEMA,
  mode: approved ? 'approved_detached_historical_report_read_only' : 'offline_contracts_only', limitations: ['Read-only'],
  capabilities: [
    { name: 'health', availability: 'available', path: '/api/v1/health' },
    { name: 'scenario_contracts', availability: 'available', path: '/api/v1/scenarios' },
    approved ? { name: 'proof_reports', availability: 'available', path: '/api/v1/proof-summary' } : { name: 'proof_reports', availability: 'not_enabled' },
    { name: 'control_findings', availability: 'not_enabled' },
    { name: 'environment_inventory', availability: 'not_enabled' },
    { name: 'scenario_execution', availability: 'not_enabled' },
    { name: 'control_catalog', availability: 'available', path: '/api/v1/control-catalog' },
    approved ? { name: 'historical_environment_snapshot', availability: 'available', path: '/api/v1/environment-snapshot' }
      : { name: 'historical_environment_snapshot', availability: 'not_enabled' },
    approved ? { name: 'historical_activity', availability: 'available', path: '/api/v1/historical-activity' }
      : { name: 'historical_activity', availability: 'not_enabled' },
      { name: 'historical_containment_map', availability: 'not_enabled' },
  ],
});
const copy = <T,>(value: T): T => JSON.parse(JSON.stringify(value)) as T;

describe('6D2 read-only observation contracts', () => {
  it('Vite forwards exactly the nine approved read-only endpoints to loopback', () => {
    const proxy = (viteConfig as UserConfig).server?.proxy;
    expect(proxy).toBeDefined();
    if (!proxy) throw new Error('Missing development proxy');
    const entries = Object.entries(proxy);
    expect(entries).toHaveLength(1);
    const [route, target] = entries[0];
    const matcher = new RegExp(route);
    for (const path of Object.values(endpoints)) {
      expect(matcher.test(path), `Missing Vite proxy for ${path}`).toBe(true);
    }
    for (const forbidden of ['/api/v1/control-findings', '/api/v1/environment-inventory', '/api/v1/proof-summary/extra', '/api/v1/execute', '/api/v1/environment-snapshot?live=true', '/api/v1/containment-map/production', '/api/v1/containment-map/hardened?live=1', '/api/v1/containment-map/baseline/extra']) {
      expect(matcher.test(forbidden), `Unexpected Vite proxy for ${forbidden}`).toBe(false);
    }
    if (typeof target === 'string') throw new Error('Proxy must strip browser credentials');
    expect(target.target).toBe('http://127.0.0.1:8787');
    expect((viteConfig as UserConfig).server?.host).toBe('127.0.0.1');
    expect((viteConfig as UserConfig).server?.strictPort).toBe(true);
  });
  it('accepts six exact Go definitions, not results', () => {
    const c = parseControlCatalog(catalogFixture());
    expect(c.definitions).toHaveLength(6);
    expect(c.evaluation_status).toBe('not_evaluated');
    expect(c.definitions[3].legacy_id).toBe('C-004');
  });
  it('rejects bogus findings, duplicate controls, changed provenance and unrecognized domains', () => {
    const altered = copy(catalogFixture());
    (altered as typeof altered & { findings?: unknown }).findings = [{ verdict: 'pass' }];
    expect(() => parseControlCatalog(altered)).toThrow();
    const duplicate = copy(catalogFixture()); duplicate.definitions[1].legacy_id = 'C-001';
    expect(() => parseControlCatalog(duplicate)).toThrow();
    const source = copy(catalogFixture()); source.source = 'live_kube_findings';
    expect(() => parseControlCatalog(source)).toThrow();
    const domain = copy(catalogFixture()); domain.definitions[0].domain = 'everything';
    expect(() => parseControlCatalog(domain)).toThrow();
  });
  it('accepts approved historical counts and rejects live/verified or raw evidence fields', () => {
    const s = parseHistoricalEnvironment(environmentFixture());
    expect(s.current_read).toBe(false);
    expect(s.enforcement_verified).toBe(false);
    for (const [field, value] of [['current_read', true], ['enforcement_verified', true], ['source', 'live_inventory'], ['profile', 'production'], ['policy_count', -1]] as const) {
      expect(() => parseHistoricalEnvironment({ ...environmentFixture(), [field]: value })).toThrow();
    }
    expect(() => parseHistoricalEnvironment({ ...environmentFixture(), terraform_state: 'unsafe' })).toThrow();
  });
  it('shows non-evaluated control cards and constrained historical observations', () => {
    const controls = renderToStaticMarkup(<ControlCatalogPanel catalog={parseControlCatalog(catalogFixture())}/>);
    expect(controls).toContain('C-001');
    expect(controls).toContain('NOT EVALUATED');
    expect(controls).not.toContain('PASSED');
    const environment = renderToStaticMarkup(<HistoricalEnvironmentPanel summary={parseHistoricalEnvironment(environmentFixture())}/>);
    expect(environment).toContain('Not a live inventory');
    expect(environment).toContain('Enforcement verified');
    expect(environment).toContain('NOT CURRENT');
  });
  it('never automatically accesses the approved historical endpoint during initial render', () => {
    const spy = vi.spyOn(globalThis, 'fetch');
    try {
      expect(renderToStaticMarkup(<ControlsView refresh={0}/>)).toContain('Loading six Go control definitions');
      const locked = renderToStaticMarkup(<EnvironmentView capabilities={capabilitiesFixture(false)} capabilityLoading={false}/>);
      expect(locked).toContain('Historical environment unavailable');
      expect(locked).not.toContain('Load historical snapshot');
      const approved = renderToStaticMarkup(<EnvironmentView capabilities={capabilitiesFixture(true)} capabilityLoading={false}/>);
      expect(approved).toContain('Load historical snapshot');
      expect(approved).not.toContain('Ready Pods');
      expect(spy).not.toHaveBeenCalled();
    } finally { spy.mockRestore(); }
  });
  it('requests only bounded JSON by safe GET without credentials; rejects oversized data', async () => {
    const spy = vi.spyOn(globalThis, 'fetch');
    try {
      spy.mockResolvedValueOnce(new Response(JSON.stringify(catalogFixture()), { headers: { 'Content-Type': 'application/json' } }));
      expect((await loadControlCatalog(new AbortController().signal)).definitions).toHaveLength(6);
      expect(spy).toHaveBeenCalledWith(endpoints.controls, expect.objectContaining({ method: 'GET', credentials: 'omit', redirect: 'error', cache: 'no-store' }));
      spy.mockResolvedValueOnce(new Response(JSON.stringify(environmentFixture()), { headers: { 'Content-Type': 'application/json' } }));
      expect((await loadHistoricalEnvironment(new AbortController().signal)).current_read).toBe(false);
      expect(spy).toHaveBeenCalledWith(endpoints.environment, expect.objectContaining({ method: 'GET', credentials: 'omit', redirect: 'error', cache: 'no-store' }));
      spy.mockResolvedValueOnce(new Response(JSON.stringify({ ...environmentFixture(), extra: 'x'.repeat(40000) }), { headers: { 'Content-Type': 'application/json' } }));
      await expect(loadHistoricalEnvironment(new AbortController().signal)).rejects.toThrow();
    } finally { spy.mockRestore(); }
  });
});
