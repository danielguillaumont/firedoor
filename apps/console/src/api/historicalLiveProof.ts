// G2: Browser-only, user-selected G1 historical proof. Never fetch private
// files, call the API, persist data, or claim current Kubernetes enforcement.
export const HISTORICAL_LIVE_PROOF_SCHEMA = 'firedoor.v10.console-live-proof-projection.v1' as const;
const SOURCE = 'explicit_private_f5_receipt_projection' as const;
const STATUS = 'historical_verified_at_capture_not_live' as const;
const LAB = 'Disposable KIND network-policy lab' as const;
export const MAX_HISTORICAL_PROOF_BYTES = 32 * 1024;

export const HISTORICAL_PROOF_LIMITATIONS = [
  'Historical F5 observations were verified at capture time; this projection does not check current cluster state.',
  'Five HTTP paths demonstrate bounded behavior in a disposable lab; they are not universal proof of isolation.',
  'A local SHA-256 match is an integrity check, not an independent or cryptographic attestation of origin.',
  'Recovery and production security posture are not established by this display projection.',
] as const;

export const HISTORICAL_PROOF_PATHS = [
  { id: 'edge-to-app', from: 'edge', to: 'app', observation: 'allowed' },
  { id: 'app-to-data', from: 'app', to: 'data', observation: 'allowed' },
  { id: 'data-to-edge-control', from: 'data', to: 'edge', observation: 'allowed' },
  { id: 'edge-to-data', from: 'edge', to: 'data', observation: 'denied' },
  { id: 'data-to-app', from: 'data', to: 'app', observation: 'denied' },
] as const;
export type ProofPathID = typeof HISTORICAL_PROOF_PATHS[number]['id'];
export type HistoricalProofPath = { id: ProofPathID; from: 'edge' | 'app' | 'data'; to: 'edge' | 'app' | 'data'; observation: 'allowed' | 'denied' };
export type HistoricalLiveProof = {
  schema_version: typeof HISTORICAL_LIVE_PROOF_SCHEMA;
  source: typeof SOURCE;
  evidence_status: typeof STATUS;
  lab_label: typeof LAB;
  observed_utc: string;
  allowed_count: 3;
  denied_count: 2;
  network_verified_at_capture: true;
  current_cluster_checked: false;
  current_enforcement_verified: false;
  terraform_executed_by_projection: false;
  kubernetes_changed_by_projection: false;
  recovery_proven: false;
  observations: HistoricalProofPath[];
  limitations: string[];
};

function record(input: unknown, name: string, keys: readonly string[]): Record<string, unknown> {
  if (input === null || typeof input !== 'object' || Array.isArray(input)) throw new Error(`${name}: expected object`);
  const value = input as Record<string, unknown>;
  const actual = Object.keys(value);
  if (actual.length !== keys.length || actual.some((key) => !keys.includes(key))) throw new Error(`${name}: unexpected fields`);
  return value;
}
function exact(value: unknown, expected: string | boolean | number, name: string) {
  if (value !== expected) throw new Error(`${name}: unexpected value`);
}

// The browser checks an allowlisted display contract; it cannot independently
// authenticate the private source receipt. Only a G1-produced projection qualifies.
export function parseHistoricalLiveProof(input: unknown): HistoricalLiveProof {
  const value = record(input, 'projection', [
    'schema_version', 'source', 'evidence_status', 'lab_label', 'observed_utc',
    'allowed_count', 'denied_count', 'network_verified_at_capture', 'current_cluster_checked',
    'current_enforcement_verified', 'terraform_executed_by_projection', 'kubernetes_changed_by_projection',
    'recovery_proven', 'observations', 'limitations',
  ]);
  exact(value.schema_version, HISTORICAL_LIVE_PROOF_SCHEMA, 'schema');
  exact(value.source, SOURCE, 'source');
  exact(value.evidence_status, STATUS, 'evidence status');
  exact(value.lab_label, LAB, 'lab label');
  exact(value.allowed_count, 3, 'allowed count');
  exact(value.denied_count, 2, 'denied count');
  exact(value.network_verified_at_capture, true, 'verified at capture');
  exact(value.current_cluster_checked, false, 'current cluster checked');
  exact(value.current_enforcement_verified, false, 'current enforcement');
  exact(value.terraform_executed_by_projection, false, 'Terraform executed');
  exact(value.kubernetes_changed_by_projection, false, 'Kubernetes changed');
  exact(value.recovery_proven, false, 'recovery status');

  const timestamp = value.observed_utc;
  if (typeof timestamp !== 'string' || timestamp.length > 48 ||
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(timestamp) ||
    !Number.isFinite(Date.parse(timestamp)) || Number.isNaN(Date.parse(timestamp)) ||
    new Date(timestamp).getUTCFullYear() < 2026) {
    throw new Error('observed_utc: invalid timestamp');
  }
  // Ensure exact calendar date, not a normalized invalid timestamp (e.g. Feb 30).
  if (new Date(timestamp).toISOString().slice(0, 19) !== timestamp.slice(0, 19)) {
    throw new Error('observed_utc: invalid calendar date');
  }
  if (!Array.isArray(value.observations) || value.observations.length !== HISTORICAL_PROOF_PATHS.length) {
    throw new Error('observations: exactly five paths required');
  }
  const paths: HistoricalProofPath[] = value.observations.map((item: unknown, i: number) => {
    const entry = record(item, `observation ${i + 1}`, ['id', 'from', 'to', 'observation']);
    const expected = HISTORICAL_PROOF_PATHS[i];
    exact(entry.id, expected.id, 'path ID');
    exact(entry.from, expected.from, 'source zone');
    exact(entry.to, expected.to, 'destination zone');
    exact(entry.observation, expected.observation, 'traffic outcome');
    return { id: expected.id, from: expected.from, to: expected.to, observation: expected.observation };
  });
  if (!Array.isArray(value.limitations) || value.limitations.length !== HISTORICAL_PROOF_LIMITATIONS.length ||
    value.limitations.some((item, i) => item !== HISTORICAL_PROOF_LIMITATIONS[i])) {
    throw new Error('limitations: published scope changed');
  }
  return {
    schema_version: HISTORICAL_LIVE_PROOF_SCHEMA,
    source: SOURCE, evidence_status: STATUS, lab_label: LAB,
    observed_utc: timestamp, allowed_count: 3, denied_count: 2,
    network_verified_at_capture: true, current_cluster_checked: false,
    current_enforcement_verified: false, terraform_executed_by_projection: false,
    kubernetes_changed_by_projection: false, recovery_proven: false,
    observations: paths, limitations: [...HISTORICAL_PROOF_LIMITATIONS],
  };
}

export function parseHistoricalLiveProofFile(contents: string): HistoricalLiveProof {
  if (new TextEncoder().encode(contents).length > MAX_HISTORICAL_PROOF_BYTES) {
    throw new Error('Projection exceeds the 32 KiB display limit');
  }
  // Permit one UTF-8 BOM caused by text export; never accept arbitrary text prefixes.
  const text = contents.charCodeAt(0) === 0xfeff ? contents.slice(1) : contents;
  return parseHistoricalLiveProof(JSON.parse(text) as unknown);
}
