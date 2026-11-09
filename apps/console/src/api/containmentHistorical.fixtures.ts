// Synthetic, test-only projections. Never saved to the Git-ignored evidence path.
import { historicalLimitations } from './containmentHistorical';
import { expectedPlanningReachability, type MapProfile } from './containment';

const routes = [
  ['edge-to-app', 'edge', 'app'], ['app-to-data', 'app', 'data'], ['edge-to-data', 'edge', 'data'],
  ['data-to-app', 'data', 'app'], ['app-to-edge', 'app', 'edge'], ['data-to-edge', 'data', 'edge'],
] as const;
export const historicalStamp = '2026-11-07T12:00:00Z';
export function historicalFixture(profile: MapProfile = 'hardened') {
  return {
    schema_version: 'firedoor.v07.containment-map.v1', cluster: 'firedoor-local', context: 'kind-firedoor-local',
    profile, endpoint_mode: 'direct-pod-ip', transport: 'HTTP/TCP', port: 8080,
    source: 'historical_projection', current_inventory: false, enforcement_verified: false,
    nodes: ['edge', 'app', 'data'].map((id) => ({ id, kind: 'zone', zone: id, namespace: `firedoor-${id}` })),
    routes: routes.map(([id, from, to], index) => {
      const result = index < 2 ? index === 0 ? 'reachable' : 'unreachable' : 'not_observed';
      return {
        id, from, to, declared: 'unspecified', observed: result,
        ...(index < 2 ? { observation_evidence_ids: [`observation-${id}`] } : {}),
        assessment: index < 2 ? 'inconclusive' : 'not_assessed',
        reason: index < 2 ? `Saved v0.3 HTTP/TCP observation recorded ${result} against a ${expectedPlanningReachability(profile, id)} probe-plan expectation. Historical connectivity is not verified policy enforcement.` :
          'Saved v0.3 probe did not establish reliable reachability. This route remains unobserved and not assessed.',
      };
    }),
    evidence: routes.slice(0, 2).map(([id], i) => ({
      id: `observation-${id}`, kind: 'network_observation', source_id: 'approved-local-v03',
      captured_at_utc: historicalStamp, profile, result: i === 0 ? 'reachable' : 'unreachable',
    })),
    limitations: [...historicalLimitations],
  };
}
