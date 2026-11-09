// Firedoor v0.7 / 7C2. Strict parser for the two *declared-only* map endpoints.
// Historical records, current inventory and enforcement verdicts are NOT
// accepted by this client contract. A future approved projection needs its
// own separately reviewed parser and access boundary.
export const CONTAINMENT_SCHEMA = 'firedoor.v07.containment-map.v1' as const;
export type MapProfile = 'baseline' | 'hardened';
export type MapZone = 'edge' | 'app' | 'data';
export type MapRouteID = 'edge-to-app' | 'app-to-data' | 'edge-to-data' | 'data-to-app' | 'app-to-edge' | 'data-to-edge';

export interface PlanningMapNode {
  id: MapZone;
  kind: 'zone';
  zone: MapZone;
  namespace: string;
}
export interface PlanningMapRoute {
  id: MapRouteID;
  from: MapZone;
  to: MapZone;
  declared: 'unspecified';
  observed: 'not_observed';
  assessment: 'not_assessed';
  reason: string;
}
export interface PlanningContainmentMap {
  schema_version: typeof CONTAINMENT_SCHEMA;
  cluster: 'firedoor-local';
  context: 'kind-firedoor-local';
  profile: MapProfile;
  endpoint_mode: 'direct-pod-ip';
  transport: 'HTTP/TCP';
  port: 8080;
  source: 'declared_contract';
  current_inventory: false;
  enforcement_verified: false;
  nodes: PlanningMapNode[];
  routes: PlanningMapRoute[];
  evidence: [];
  limitations: string[];
}

const zones: readonly MapZone[] = ['edge', 'app', 'data'];
const routeMap: Readonly<Record<MapRouteID, readonly [MapZone, MapZone]>> = {
  'edge-to-app': ['edge', 'app'],
  'app-to-data': ['app', 'data'],
  'edge-to-data': ['edge', 'data'],
  'data-to-app': ['data', 'app'],
  'app-to-edge': ['app', 'edge'],
  'data-to-edge': ['data', 'edge'],
};
const routeIDs = Object.keys(routeMap) as MapRouteID[];
const planningLimitations = [
  'Zone nodes and routes are a validated fixed lab plan, not a current Kubernetes workload inventory.',
  'Probe expectations are planning-only; no Terraform plan receipt establishes declared policy intent.',
  'No network observations or enforcement checks were performed; every route remains not assessed.',
];

function record(value: unknown, context: string): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`${context}: expected object`);
  }
  return value as Record<string, unknown>;
}
function fields(value: Record<string, unknown>, expected: readonly string[], context: string): void {
  const allowed = new Set(expected);
  if (Object.keys(value).length !== allowed.size || Object.keys(value).some((key) => !allowed.has(key))) {
    throw new Error(`${context}: unexpected or missing fields`);
  }
}
function equal(actual: unknown, expected: unknown, context: string): void {
  if (actual !== expected) throw new Error(`${context}: unexpected value`);
}
function array(value: unknown, expectedLength: number, context: string): unknown[] {
  if (!Array.isArray(value) || value.length !== expectedLength) {
    throw new Error(`${context}: unexpected collection size`);
  }
  return value as unknown[];
}
function profileIsValid(value: unknown): value is MapProfile {
  return value === 'baseline' || value === 'hardened';
}
function zoneIsValid(value: unknown): value is MapZone {
  return zones.some((zone) => zone === value);
}
function routeIsValid(value: unknown): value is MapRouteID {
  return routeIDs.some((route) => route === value);
}
function expectedReachability(profile: MapProfile, id: MapRouteID): 'reachable' | 'unreachable' {
  return profile === 'hardened' && (id === 'edge-to-data' || id === 'data-to-app') ? 'unreachable' : 'reachable';
}
function expectedReason(profile: MapProfile, id: MapRouteID): string {
  return `The validated v0.3 plan expects ${expectedReachability(profile, id)} HTTP/TCP connectivity. This is planning-only, not a Terraform-backed declaration, measured result, or enforcement proof.`;
}

// The route and node allowlists must match the reviewed Go 7C1 projection,
// including the absence of optional evidence fields (Go json:omitempty).
// Do not generalize this parser to historical maps without a separate review.
export function parsePlanningContainmentMap(value: unknown, requestedProfile: MapProfile): PlanningContainmentMap {
  if (!profileIsValid(requestedProfile)) throw new Error('Unapproved containment map profile');
  const v = record(value, 'containment map');
  fields(v, ['schema_version', 'cluster', 'context', 'profile', 'endpoint_mode', 'transport', 'port',
    'source', 'current_inventory', 'enforcement_verified', 'nodes', 'routes', 'evidence', 'limitations'], 'containment map');
  equal(v.schema_version, CONTAINMENT_SCHEMA, 'schema_version');
  equal(v.cluster, 'firedoor-local', 'cluster');
  equal(v.context, 'kind-firedoor-local', 'context');
  equal(v.profile, requestedProfile, 'profile');
  equal(v.endpoint_mode, 'direct-pod-ip', 'endpoint_mode');
  equal(v.transport, 'HTTP/TCP', 'transport');
  equal(v.port, 8080, 'port');
  equal(v.source, 'declared_contract', 'source');
  equal(v.current_inventory, false, 'current_inventory');
  equal(v.enforcement_verified, false, 'enforcement_verified');

  const seenNodes = new Set<MapZone>();
  const nodes = array(v.nodes, zones.length, 'nodes').map((item): PlanningMapNode => {
    const node = record(item, 'node');
    fields(node, ['id', 'kind', 'zone', 'namespace'], 'node');
    if (!zoneIsValid(node.id) || seenNodes.has(node.id)) throw new Error('Duplicate or unsupported map node');
    equal(node.kind, 'zone', 'node.kind');
    equal(node.zone, node.id, 'node.zone');
    equal(node.namespace, `firedoor-${node.id}`, 'node.namespace');
    seenNodes.add(node.id);
    return { id: node.id, kind: 'zone', zone: node.id, namespace: `firedoor-${node.id}` };
  });

  const seenRoutes = new Set<MapRouteID>();
  const routes = array(v.routes, routeIDs.length, 'routes').map((item): PlanningMapRoute => {
    const route = record(item, 'route');
    fields(route, ['id', 'from', 'to', 'declared', 'observed', 'assessment', 'reason'], 'route');
    if (!routeIsValid(route.id) || seenRoutes.has(route.id)) throw new Error('Duplicate or unsupported map route');
    seenRoutes.add(route.id);
    const [from, to] = routeMap[route.id];
    equal(route.from, from, 'route.from');
    equal(route.to, to, 'route.to');
    equal(route.declared, 'unspecified', 'route.declared');
    equal(route.observed, 'not_observed', 'route.observed');
    equal(route.assessment, 'not_assessed', 'route.assessment');
    equal(route.reason, expectedReason(requestedProfile, route.id), 'route.reason');
    return { id: route.id, from, to, declared: 'unspecified', observed: 'not_observed', assessment: 'not_assessed', reason: expectedReason(requestedProfile, route.id) };
  });
  array(v.evidence, 0, 'evidence');
  const limitations = array(v.limitations, planningLimitations.length, 'limitations').map((item, i) => {
    equal(item, planningLimitations[i], `limitations[${i}]`);
    return planningLimitations[i];
  });
  return {
    schema_version: CONTAINMENT_SCHEMA, cluster: 'firedoor-local', context: 'kind-firedoor-local',
    profile: requestedProfile, endpoint_mode: 'direct-pod-ip', transport: 'HTTP/TCP', port: 8080,
    source: 'declared_contract', current_inventory: false, enforcement_verified: false,
    nodes, routes, evidence: [], limitations,
  };
}
