// H3: strict, browser-only H2 historical scenario result display contract.
// User-selected JSON only: no fetch, persistence, Kubernetes, or certification.
export const SCENARIO_RESULT_SCHEMA = 'firedoor.v10.scenario-result.v1' as const;
export const MAX_SCENARIO_RESULT_BYTES = 32 * 1024;
export const FD004_CLAIM = 'Frontend cannot directly reach protected data service after hardening';

export const FD004_REQUIREMENTS = [
  { id: 'scenario_implemented', status: 'not_met', detail: 'FD-004 remains spec_only_not_implemented.' },
  { id: 'contract_cluster_scope', status: 'unverified', detail: "The sanitized projection does not identify the contract's kind-firedoor-local cluster." },
  { id: 'evidence_origin', status: 'unverified', detail: 'The private F5 receipt is not independently authenticated by this review.' },
  { id: 'current_cluster_assessment', status: 'not_performed', detail: 'No current cluster inspection or traffic tests were executed.' },
] as const;
export const FD004_LIMITATIONS = [
  'Historical traffic observations support one bounded assertion, not a complete scenario assessment.',
  'The FD-004 cluster scope and source receipt have not been independently verified.',
  'Mapped controls C-004 and C-005 were not evaluated by this review.',
  'This result does not establish current enforcement, policy causation, recovery, or production security.',
] as const;

export type HistoricalScenarioResult = {
  schema_version: typeof SCENARIO_RESULT_SCHEMA;
  scenario_id: 'FD-004';
  scenario_claim: typeof FD004_CLAIM;
  outcome: 'not_evaluated';
  evidence_relation: 'historical_observation_consistent_with_claim_at_capture';
  evidence_source: 'user_supplied_hash_checked_h1_review_unattested';
  observed_utc: string;
  supporting_routes: ['edge-to-data'];
  positive_controls: ['edge-to-app', 'app-to-data', 'data-to-edge-control'];
  mapped_controls_not_evaluated: ['C-004', 'C-005'];
  requirements: Array<{id: string; status: string; detail: string}>;
  scenario_evaluation_executed: false;
  contract_scope_verified: false;
  current_cluster_checked: false;
  original_receipt_authenticated: false;
  scenario_fully_certified: false;
  limitations: string[];
};

function strictObject(value: unknown, label: string, allowed: readonly string[]): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${label}: expected object`);
  const object = value as Record<string, unknown>;
  const keys = Object.keys(object);
  if (keys.length !== allowed.length || keys.some((key) => !allowed.includes(key))) {
    throw new Error(`${label}: unexpected or missing fields`);
  }
  return object;
}

function equal(value: unknown, expected: string | boolean, label: string): void {
  if (value !== expected) throw new Error(`${label}: unsupported value`);
}

function exactStrings(value: unknown, expected: readonly string[], label: string): string[] {
  if (!Array.isArray(value) || value.length !== expected.length ||
      value.some((entry, index) => entry !== expected[index])) {
    throw new Error(`${label}: unexpected or reordered evidence`);
  }
  return [...expected];
}

export function parseHistoricalScenarioResult(input: unknown): HistoricalScenarioResult {
  const fields = [
    'schema_version', 'scenario_id', 'scenario_claim', 'outcome', 'evidence_relation',
    'evidence_source', 'observed_utc', 'supporting_routes', 'positive_controls',
    'mapped_controls_not_evaluated', 'requirements', 'scenario_evaluation_executed',
    'contract_scope_verified', 'current_cluster_checked', 'original_receipt_authenticated',
    'scenario_fully_certified', 'limitations',
  ];
  const result = strictObject(input, 'scenario result', fields);
  equal(result.schema_version, SCENARIO_RESULT_SCHEMA, 'schema');
  equal(result.scenario_id, 'FD-004', 'scenario');
  equal(result.scenario_claim, FD004_CLAIM, 'scenario claim');
  equal(result.outcome, 'not_evaluated', 'outcome');
  equal(result.evidence_relation, 'historical_observation_consistent_with_claim_at_capture', 'relation');
  equal(result.evidence_source, 'user_supplied_hash_checked_h1_review_unattested', 'source');

  for (const key of [
    'scenario_evaluation_executed', 'contract_scope_verified', 'current_cluster_checked',
    'original_receipt_authenticated', 'scenario_fully_certified',
  ]) equal(result[key], false, key);

  const timestamp = result.observed_utc;
  if (typeof timestamp !== 'string' || timestamp.length > 48 ||
      !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(timestamp) ||
      !Number.isFinite(Date.parse(timestamp)) ||
      new Date(timestamp).getUTCFullYear() < 2026 ||
      new Date(timestamp).toISOString().slice(0, 19) !== timestamp.slice(0, 19)) {
    throw new Error('observed_utc: invalid UTC capture time');
  }

  exactStrings(result.supporting_routes, ['edge-to-data'], 'supporting routes');
  exactStrings(result.positive_controls, ['edge-to-app', 'app-to-data', 'data-to-edge-control'], 'positive controls');
  exactStrings(result.mapped_controls_not_evaluated, ['C-004', 'C-005'], 'mapped controls');
  exactStrings(result.limitations, FD004_LIMITATIONS, 'limitations');
  if (!Array.isArray(result.requirements) || result.requirements.length !== FD004_REQUIREMENTS.length) {
    throw new Error('requirements: exactly four unresolved gates required');
  }
  result.requirements.forEach((item: unknown, index: number) => {
    const gate = strictObject(item, `requirement ${index + 1}`, ['id', 'status', 'detail']);
    const expected = FD004_REQUIREMENTS[index];
    equal(gate.id, expected.id, 'requirement ID');
    equal(gate.status, expected.status, 'requirement status');
    equal(gate.detail, expected.detail, 'requirement detail');
  });

  // Return a minimized display model, never propagate arbitrary source objects.
  return {
    schema_version: SCENARIO_RESULT_SCHEMA,
    scenario_id: 'FD-004',
    scenario_claim: FD004_CLAIM,
    outcome: 'not_evaluated',
    evidence_relation: 'historical_observation_consistent_with_claim_at_capture',
    evidence_source: 'user_supplied_hash_checked_h1_review_unattested',
    observed_utc: timestamp,
    supporting_routes: ['edge-to-data'],
    positive_controls: ['edge-to-app', 'app-to-data', 'data-to-edge-control'],
    mapped_controls_not_evaluated: ['C-004', 'C-005'],
    requirements: FD004_REQUIREMENTS.map((item) => ({ ...item })),
    scenario_evaluation_executed: false,
    contract_scope_verified: false,
    current_cluster_checked: false,
    original_receipt_authenticated: false,
    scenario_fully_certified: false,
    limitations: [...FD004_LIMITATIONS],
  };
}

export function parseHistoricalScenarioResultFile(contents: string): HistoricalScenarioResult {
  if (new TextEncoder().encode(contents).length > MAX_SCENARIO_RESULT_BYTES) {
    throw new Error('H2 result exceeds 32 KiB');
  }
  const text = contents.charCodeAt(0) === 0xfeff ? contents.slice(1) : contents;
  return parseHistoricalScenarioResult(JSON.parse(text) as unknown);
}
