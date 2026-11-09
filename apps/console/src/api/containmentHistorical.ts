// 7E2B: separate, deliberately narrow browser contract for Go's consent-gated
// historical projection. Never use the planning parser for observed records.
// Accepting a structurally consistent projection does NOT authenticate the file
// origin or establish Terraform application / NetworkPolicy enforcement.
import { CONTAINMENT_SCHEMA, expectedPlanningReachability } from './containment';
import type { MapProfile, MapRouteID, MapZone } from './containment';
import type { Capabilities } from './contracts';

export type RecordedReachability = 'reachable' | 'unreachable' | 'not_observed';
export interface HistoricalMapRoute {
  id: MapRouteID;
  from: MapZone;
  to: MapZone;
  observed: RecordedReachability;
  assessment: 'inconclusive' | 'not_assessed';
  captured_utc: string | null;
}
export interface HistoricalContainmentMap {
  profile: MapProfile;
  source: 'historical_projection';
  routes: HistoricalMapRoute[];
  observed_count: number;
  limitations: readonly string[];
}

export const historicalLimitations = [
  'Approved saved v0.3 observation only, not current inventory or a live network scan.',
  'Original file origin and operator-supplied profile are not independently authenticated.',
  'Point-in-time HTTP connectivity does not prove NetworkPolicy causation or Terraform application.',
  'Unresolved routes are not observed or assessed; observed routes remain inconclusive for enforcement.',
  'Evidence source labels are deliberately generic; timestamps are historical capture times.',
] as const;

const routePairs: Readonly<Record<MapRouteID, readonly [MapZone, MapZone]>> = {
  'edge-to-app': ['edge', 'app'], 'app-to-data': ['app', 'data'],
  'edge-to-data': ['edge', 'data'], 'data-to-app': ['data', 'app'],
  'app-to-edge': ['app', 'edge'], 'data-to-edge': ['data', 'edge'],
};
const routeIDs = Object.keys(routePairs) as MapRouteID[];
const zones: readonly MapZone[] = ['edge', 'app', 'data'];

function object(input: unknown, label: string): Record<string, unknown> {
  if (!input || typeof input !== 'object' || Array.isArray(input)) throw new Error(`${label}: expected object`);
  return input as Record<string, unknown>;
}
function exact(value: Record<string, unknown>, names: readonly string[], label: string): void {
  if (Object.keys(value).length !== names.length || Object.keys(value).some((key) => !names.includes(key))) {
    throw new Error(`${label}: unknown or missing fields`);
  }
}
function equal(value: unknown, expected: unknown, label: string): void {
  if (value !== expected) throw new Error(`${label}: invalid value`);
}
function items(value: unknown, length: number, label: string): unknown[] {
  if (!Array.isArray(value) || value.length !== length) throw new Error(`${label}: invalid length`);
  return value as unknown[];
}
function profile(value: unknown): value is MapProfile { return value === 'baseline' || value === 'hardened'; }
function routeID(value: unknown): value is MapRouteID {
  return typeof value === 'string' && Object.prototype.hasOwnProperty.call(routePairs, value);
}
function measured(value: unknown): value is Exclude<RecordedReachability, 'not_observed'> {
  return value === 'reachable' || value === 'unreachable';
}
function timestamp(value: unknown): string {
  // Go's time.Time JSON encoder emits UTC RFC3339 with optional fractional seconds.
  if (typeof value !== 'string' || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(value) || Number.isNaN(Date.parse(value)) ||
    new Date(value).toISOString().slice(0, 19) !== value.slice(0, 19) || value.startsWith('0001-01-01T00:00:00')) {
    throw new Error('historical timestamp: invalid UTC time');
  }
  return value;
}

export function isHistoricalMapApproved(capabilities: Capabilities | null): boolean {
  return capabilities?.mode === 'approved_v07_historical_map_read_only' &&
    capabilities.capabilities.some((entry) => entry.name === 'historical_containment_map' &&
      entry.availability === 'available' && entry.path === '/api/v1/containment-map/historical');
}

export function parseHistoricalContainmentMap(input: unknown): HistoricalContainmentMap {
  const map = object(input, 'historical map');
  exact(map, ['schema_version', 'cluster', 'context', 'profile', 'endpoint_mode', 'transport', 'port',
    'source', 'current_inventory', 'enforcement_verified', 'nodes', 'routes', 'evidence', 'limitations'], 'historical map');
  equal(map.schema_version, CONTAINMENT_SCHEMA, 'schema');
  equal(map.cluster, 'firedoor-local', 'cluster');
  equal(map.context, 'kind-firedoor-local', 'context');
  if (!profile(map.profile)) throw new Error('historical map: unapproved profile');
  const mapProfile = map.profile;
  equal(map.endpoint_mode, 'direct-pod-ip', 'endpoint mode');
  equal(map.transport, 'HTTP/TCP', 'transport');
  equal(map.port, 8080, 'port');
  equal(map.source, 'historical_projection', 'source');
  equal(map.current_inventory, false, 'current inventory');
  equal(map.enforcement_verified, false, 'enforcement');

  const seenZones = new Set<MapZone>();
  items(map.nodes, 3, 'nodes').forEach((item) => {
    const node = object(item, 'zone node');
    exact(node, ['id', 'kind', 'zone', 'namespace'], 'zone node');
    if (typeof node.id !== 'string' || !zones.includes(node.id as MapZone) || seenZones.has(node.id as MapZone)) throw new Error('unsupported or duplicate zone');
    const id = node.id as MapZone;
    equal(node.kind, 'zone', 'node kind');
    equal(node.zone, id, 'node zone');
    equal(node.namespace, `firedoor-${id}`, 'namespace');
    seenZones.add(id);
  });

  const evidenceByID = new Map<string, { result: Exclude<RecordedReachability, 'not_observed'>; captured: string }>();
  const rawEvidence = map.evidence;
  if (!Array.isArray(rawEvidence) || rawEvidence.length > 6) throw new Error('evidence: exceeds fixed scope');
  rawEvidence.forEach((item) => {
    const evidence = object(item, 'network evidence');
    exact(evidence, ['id', 'kind', 'source_id', 'captured_at_utc', 'profile', 'result'], 'network evidence');
    if (typeof evidence.id !== 'string' || !evidence.id.startsWith('observation-') ||
      !routeID(evidence.id.slice('observation-'.length)) || evidenceByID.has(evidence.id)) throw new Error('unsupported evidence identity');
    equal(evidence.kind, 'network_observation', 'evidence kind');
    equal(evidence.source_id, 'approved-local-v03', 'evidence source');
    equal(evidence.profile, mapProfile, 'evidence profile');
    if (!measured(evidence.result)) throw new Error('unmeasured evidence result');
    evidenceByID.set(evidence.id, { result: evidence.result, captured: timestamp(evidence.captured_at_utc) });
  });

  const seenRoutes = new Set<MapRouteID>();
  const usedEvidence = new Set<string>();
  const routes = items(map.routes, routeIDs.length, 'routes').map((item): HistoricalMapRoute => {
    const route = object(item, 'historical route');
    const hasEvidence = route.observation_evidence_ids !== undefined;
    exact(route, hasEvidence ? ['id', 'from', 'to', 'declared', 'observed', 'observation_evidence_ids', 'assessment', 'reason'] :
      ['id', 'from', 'to', 'declared', 'observed', 'assessment', 'reason'], 'historical route');
    if (!routeID(route.id) || seenRoutes.has(route.id)) throw new Error('duplicate or unsupported route');
    const id = route.id;
    seenRoutes.add(id);
    const [from, to] = routePairs[id];
    equal(route.from, from, 'from'); equal(route.to, to, 'to');
    equal(route.declared, 'unspecified', 'unverified declaration');
    if (route.observed === 'not_observed') {
      equal(route.assessment, 'not_assessed', 'unobserved assessment');
      equal(hasEvidence, false, 'unobserved evidence');
      equal(route.reason, 'Saved v0.3 probe did not establish reliable reachability. This route remains unobserved and not assessed.', 'unobserved reason');
      return { id, from, to, observed: 'not_observed', assessment: 'not_assessed', captured_utc: null };
    }
    if (!measured(route.observed)) throw new Error('unsupported historical observation');
    equal(route.assessment, 'inconclusive', 'observed assessment');
    const ref = `observation-${id}`;
    const ids = items(route.observation_evidence_ids, 1, 'observation references');
    equal(ids[0], ref, 'observation reference');
    if (usedEvidence.has(ref)) throw new Error('reused evidence');
    const source = evidenceByID.get(ref);
    if (!source || source.result !== route.observed) throw new Error('missing or conflicting evidence');
    usedEvidence.add(ref);
    equal(route.reason, `Saved v0.3 HTTP/TCP observation recorded ${route.observed} against a ${expectedPlanningReachability(mapProfile, id)} probe-plan expectation. Historical connectivity is not verified policy enforcement.`, 'observation reason');
    return { id, from, to, observed: route.observed, assessment: 'inconclusive', captured_utc: source.captured };
  });
  if (usedEvidence.size !== evidenceByID.size) throw new Error('unreferenced evidence');
  const limits = items(map.limitations, historicalLimitations.length, 'limitations');
  limits.forEach((item, i) => equal(item, historicalLimitations[i], `limitation ${i}`));
  return { profile: mapProfile, source: 'historical_projection', routes,
    observed_count: usedEvidence.size, limitations: historicalLimitations };
}
