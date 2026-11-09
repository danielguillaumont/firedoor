import { afterEach, describe, expect, it, vi } from 'vitest';
import type { UserConfig } from 'vite';
import viteConfig from '../../vite.config';
import { endpoints, loadPlanningContainmentMap } from './client';
import { CONTAINMENT_SCHEMA, parsePlanningContainmentMap, type MapProfile } from './containment';

const pairs = [
  ['edge-to-app', 'edge', 'app'],
  ['app-to-data', 'app', 'data'],
  ['edge-to-data', 'edge', 'data'],
  ['data-to-app', 'data', 'app'],
  ['app-to-edge', 'app', 'edge'],
  ['data-to-edge', 'data', 'edge'],
] as const;
const limitations = [
  'Zone nodes and routes are a validated fixed lab plan, not a current Kubernetes workload inventory.',
  'Probe expectations are planning-only; no Terraform plan receipt establishes declared policy intent.',
  'No network observations or enforcement checks were performed; every route remains not assessed.',
];
const reason = (profile: MapProfile, id: string) => {
  const outcome = profile === 'hardened' && (id === 'edge-to-data' || id === 'data-to-app') ? 'unreachable' : 'reachable';
  return `The validated v0.3 plan expects ${outcome} HTTP/TCP connectivity. This is planning-only, not a Terraform-backed declaration, measured result, or enforcement proof.`;
};
const fixture = (profile: MapProfile) => ({
  schema_version: CONTAINMENT_SCHEMA,
  cluster: 'firedoor-local', context: 'kind-firedoor-local', profile,
  endpoint_mode: 'direct-pod-ip', transport: 'HTTP/TCP', port: 8080,
  source: 'declared_contract', current_inventory: false, enforcement_verified: false,
  nodes: ['edge', 'app', 'data'].map((id) => ({ id, kind: 'zone', zone: id, namespace: `firedoor-${id}` })),
  routes: pairs.map(([id, from, to]) => ({ id, from, to, declared: 'unspecified', observed: 'not_observed', assessment: 'not_assessed', reason: reason(profile, id) })),
  evidence: [], limitations: [...limitations],
});
const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value)) as T;
const response = (body: unknown) => new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } });

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers(); });

describe('7C2 strict planning-only Containment Map integration', () => {
  it.each(['baseline', 'hardened'] as const)('accepts the reviewed %s map only as a planning contract', (profile) => {
    const result = parsePlanningContainmentMap(fixture(profile), profile);
    expect(result.nodes).toHaveLength(3);
    expect(result.routes).toHaveLength(6);
    expect(result.evidence).toEqual([]);
    expect(result.current_inventory).toBe(false);
    expect(result.enforcement_verified).toBe(false);
    expect(result.routes.every((route) => route.declared === 'unspecified' && route.observed === 'not_observed' && route.assessment === 'not_assessed')).toBe(true);
    expect(result.routes.find((route) => route.id === 'edge-to-data')?.reason).toContain(profile === 'hardened' ? 'expects unreachable' : 'expects reachable');
  });

  it('rejects unapproved, switched and mixed profiles', () => {
    expect(() => parsePlanningContainmentMap(fixture('baseline'), 'hardened')).toThrow();
    expect(() => parsePlanningContainmentMap(fixture('hardened'), 'production' as MapProfile)).toThrow();
    const mixed = fixture('hardened'); mixed.routes[2].reason = reason('baseline', 'edge-to-data');
    expect(() => parsePlanningContainmentMap(mixed, 'hardened')).toThrow();
  });

  it('rejects unknown fields and unapproved cluster, context, transport, port and schema', () => {
    for (const update of [
      { schema_version: 'firedoor.v07.containment-map.v2' }, { cluster: 'production' },
      { context: 'my-cluster' }, { endpoint_mode: 'service-dns' }, { transport: 'UDP' },
      { port: 443 }, { source: 'historical_projection' }, { current_inventory: true },
      { enforcement_verified: true }, { policy_enforced: true }, { kubeconfig: 'secret' },
    ]) expect(() => parsePlanningContainmentMap({ ...fixture('baseline'), ...update }, 'baseline')).toThrow();
  });

  it('rejects extra Pod identities, raw IPs, dangling zone references and duplicate nodes', () => {
    const extra = fixture('baseline');
    expect(() => parsePlanningContainmentMap({ ...extra, nodes: [...extra.nodes, { id: 'other' }] }, 'baseline')).toThrow();
    const ip = clone(extra); Object.assign(ip.nodes[0], { pod_ip: '10.0.0.3' });
    expect(() => parsePlanningContainmentMap(ip, 'baseline')).toThrow();
    const duplicate = clone(extra); duplicate.nodes[2] = clone(duplicate.nodes[0]);
    expect(() => parsePlanningContainmentMap(duplicate, 'baseline')).toThrow();
    const scope = clone(extra); scope.nodes[0].namespace = 'kube-system';
    expect(() => parsePlanningContainmentMap(scope, 'baseline')).toThrow();
  });

  it('rejects a partial, repeated or remapped six-route matrix', () => {
    const base = fixture('hardened');
    expect(() => parsePlanningContainmentMap({ ...base, routes: base.routes.slice(1) }, 'hardened')).toThrow();
    const duplicate = clone(base); duplicate.routes[1] = clone(duplicate.routes[0]);
    expect(() => parsePlanningContainmentMap(duplicate, 'hardened')).toThrow();
    const remapped = clone(base); remapped.routes[0].to = 'data';
    expect(() => parsePlanningContainmentMap(remapped, 'hardened')).toThrow();
    const unknown = clone(base); Object.assign(unknown.routes[0], { id: 'edge-to-internet' });
    expect(() => parsePlanningContainmentMap(unknown, 'hardened')).toThrow();
  });

  it('rejects invented findings, intent receipts and historical evidence', () => {
    const base = fixture('hardened');
    for (const update of [
      { declared: 'expected_unreachable' }, { observed: 'unreachable' },
      { assessment: 'verified' }, { declared_evidence_id: 'tf-plan' },
      { observation_evidence_ids: ['abc'] }, { policy_verdict: 'pass' },
      { reason: 'NetworkPolicy enforcement verified' },
    ]) {
      const data = clone(base);
      Object.assign(data.routes[0], update);
      expect(() => parsePlanningContainmentMap(data, 'hardened')).toThrow();
    }
    expect(() => parsePlanningContainmentMap({ ...base, evidence: [{ result: 'reachable' }] }, 'hardened')).toThrow();
  });

  it('rejects altered or missing security limitations, missing fields and malformed JSON-shaped data', () => {
    const base = fixture('baseline');
    const limitation = clone(base); limitation.limitations[0] = 'This map proves protection';
    expect(() => parsePlanningContainmentMap(limitation, 'baseline')).toThrow();
    const missing = clone(base) as unknown as Record<string, unknown>; delete missing.routes;
    expect(() => parsePlanningContainmentMap(missing, 'baseline')).toThrow();
    expect(() => parsePlanningContainmentMap(null, 'baseline')).toThrow();
    expect(() => parsePlanningContainmentMap([], 'baseline')).toThrow();
  });

  it('allows safe node/route ordering without changing meaning', () => {
    const original = fixture('baseline');
    original.nodes.reverse(); original.routes.reverse();
    const parsed = parsePlanningContainmentMap(original, 'baseline');
    expect(parsed.routes).toHaveLength(6);
    expect(new Set(parsed.routes.map((r) => r.id)).size).toBe(6);
  });

  it('routes both profiles only through two explicit no-credential GET requests', async () => {
    const spy = vi.spyOn(globalThis, 'fetch');
    for (const profile of ['baseline', 'hardened'] as const) {
      spy.mockResolvedValueOnce(response(fixture(profile)));
      const loaded = await loadPlanningContainmentMap(profile, new AbortController().signal);
      expect(loaded.profile).toBe(profile);
      expect(loaded.evidence).toEqual([]);
      expect(spy).toHaveBeenLastCalledWith(profile === 'baseline' ? endpoints.baselineMap : endpoints.hardenedMap, expect.objectContaining({
        method: 'GET', credentials: 'omit', cache: 'no-store', redirect: 'error', headers: { Accept: 'application/json' },
      }));
    }
    await expect(loadPlanningContainmentMap('production' as MapProfile, new AbortController().signal)).rejects.toThrow(/Unapproved/);
    expect(spy).toHaveBeenCalledTimes(2);
  });

  it('fails closed on oversized, redirected, non-JSON and unsupported API results', async () => {
    const spy = vi.spyOn(globalThis, 'fetch');
    spy.mockResolvedValueOnce(response({ ...fixture('baseline'), padding: 'x'.repeat(33000) }));
    await expect(loadPlanningContainmentMap('baseline', new AbortController().signal)).rejects.toThrow(/size limit/);
    spy.mockResolvedValueOnce(new Response('unavailable', { status: 503 }));
    await expect(loadPlanningContainmentMap('baseline', new AbortController().signal)).rejects.toThrow(/HTTP 503/);
    spy.mockResolvedValueOnce(new Response('<html></html>', { headers: { 'Content-Type': 'text/html' } }));
    await expect(loadPlanningContainmentMap('baseline', new AbortController().signal)).rejects.toThrow(/did not return JSON/);
    spy.mockResolvedValueOnce(response(fixture('baseline')));
    await expect(loadPlanningContainmentMap('hardened', new AbortController().signal)).rejects.toThrow(/profile/);
  });

  it('propagates caller cancellation rather than showing stale route data', async () => {
    vi.spyOn(globalThis, 'fetch').mockImplementation((_path, options) => new Promise<Response>((_resolve, reject) => {
      options?.signal?.addEventListener('abort', () => reject(new DOMException('cancelled', 'AbortError')), { once: true });
    }));
    const controller = new AbortController();
    const pending = loadPlanningContainmentMap('baseline', controller.signal);
    const rejected = expect(pending).rejects.toThrow();
    controller.abort();
    await rejected;
  });

  it('Vite proxies exactly the nine explicit routes and no query, wildcard or live map URL', () => {
    const proxy = (viteConfig as UserConfig).server?.proxy;
    expect(proxy).toBeDefined();
    if (!proxy) throw new Error('Missing Vite proxy');
    const entries = Object.entries(proxy);
    expect(entries).toHaveLength(1);
    const [pattern, target] = entries[0];
    const matcher = new RegExp(pattern);
    for (const path of Object.values(endpoints)) expect(matcher.test(path), `not proxied: ${path}`).toBe(true);
    for (const forbidden of [
      '/api/v1/containment-map', '/api/v1/containment-map/production', '/api/v1/containment-map/live',
      '/api/v1/containment-map/history', '/api/v1/containment-map/hardened/export',
      '/api/v1/containment-map/baseline?profile=production', '/api/v1/terraform/apply', '/api/v1/execute',
    ]) expect(matcher.test(forbidden), `unexpectedly proxied: ${forbidden}`).toBe(false);
    if (typeof target === 'string') throw new Error('Expected no-credential proxy configuration');
    expect(target.target).toBe('http://127.0.0.1:8787');
    expect((viteConfig as UserConfig).server?.host).toBe('127.0.0.1');
    expect((viteConfig as UserConfig).server?.strictPort).toBe(true);
  });
});
