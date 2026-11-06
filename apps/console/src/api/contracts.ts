// These are deliberately strict *runtime* checks, not just TypeScript casts.
// v0.6 Slice 6B only accepts the known 6A contract-only API schema.
export const SCHEMA = 'firedoor.v06.console-api.v1' as const;

export interface Health {
  schema_version: typeof SCHEMA;
  service: 'firedoor-console-api';
  status: 'ok';
  scope: 'local_http_process_only';
  cluster_checked: false;
  evidence_loaded: false;
}

export interface Capability {
  name: string;
  path?: string;
  availability: 'available' | 'not_enabled';
}

export interface Capabilities {
  schema_version: typeof SCHEMA;
  mode: 'offline_contracts_only' | 'approved_detached_historical_report_read_only';
  capabilities: Capability[];
  limitations: string[];
}

export interface Scenario {
  schema_version: '1.0.0';
  id: string;
  name: string;
  claim: string;
  mapped_controls: string[];
  contract_status: 'spec_only_not_implemented';
  security_verdict: 'not_tested';
  evidence_loaded: false;
}

export interface Scenarios {
  schema_version: typeof SCHEMA;
  source: 'versioned_planning_contracts_only';
  scenarios: Scenario[];
  limitations: string[];
}

function object(value: unknown, context: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${context}: expected object`);
  return value as Record<string, unknown>;
}
// Proof responses are intentionally a *minimized* allowlisted JSON shape.
// Do not silently accept extra raw-evidence fields from a changed backend.
function exactFields(value: Record<string, unknown>, expected: readonly string[], context: string): void {
  const allowed = new Set(expected);
  if (Object.keys(value).length !== allowed.size || Object.keys(value).some((key) => !allowed.has(key))) {
    throw new Error(`${context}: unexpected or missing fields`);
  }
}
function text(value: unknown, context: string): string {
  if (typeof value !== 'string' || value.length < 1 || value.length > 1000) throw new Error(`${context}: invalid text`);
  return value;
}
function strings(value: unknown, context: string): string[] {
  if (!Array.isArray(value) || value.length > 30) throw new Error(`${context}: invalid collection`);
  return value.map((item, i) => text(item, `${context}[${i}]`));
}
function eq(value: unknown, expected: string | boolean | number, context: string): void {
  if (value !== expected) throw new Error(`${context}: unexpected value`);
}
function version(v: Record<string, unknown>): void { eq(v.schema_version, SCHEMA, 'schema_version'); }

export function parseHealth(value: unknown): Health {
  const v = object(value, 'health');
  version(v);
  eq(v.service, 'firedoor-console-api', 'health.service');
  eq(v.status, 'ok', 'health.status');
  eq(v.scope, 'local_http_process_only', 'health.scope');
  eq(v.cluster_checked, false, 'health.cluster_checked');
  eq(v.evidence_loaded, false, 'health.evidence_loaded');
  return v as unknown as Health;
}

const availableNames = new Set(['health', 'scenario_contracts', 'control_catalog']);
const disabledNames = new Set(['proof_reports', 'control_findings', 'environment_inventory', 'scenario_execution', 'historical_environment_snapshot']);

export function parseCapabilities(value: unknown): Capabilities {
  const v = object(value, 'capabilities');
  version(v);
  const approved = v.mode === 'approved_detached_historical_report_read_only';
  if (!approved) eq(v.mode, 'offline_contracts_only', 'capabilities.mode');
  if (!Array.isArray(v.capabilities) || v.capabilities.length !== 8) throw new Error('capabilities: unexpected count');
  const seen = new Set<string>();
  const capabilities = v.capabilities.map((item: unknown, i: number): Capability => {
    const entry = object(item, `capabilities[${i}]`);
    const name = text(entry.name, 'capability.name');
    if (seen.has(name)) throw new Error(`Duplicate capability: ${name}`);
    seen.add(name);
    if (availableNames.has(name)) {
      eq(entry.availability, 'available', `capability.${name}`);
      const expectedPath = name === 'health' ? '/api/v1/health' : name === 'scenario_contracts' ? '/api/v1/scenarios' : '/api/v1/control-catalog';
      eq(entry.path, expectedPath, `capability.${name}.path`);
    } else if (name === 'historical_environment_snapshot' && approved) {
      eq(entry.availability, 'available', 'capability.historical_environment_snapshot');
      eq(entry.path, '/api/v1/environment-snapshot', 'capability.historical_environment_snapshot.path');
    } else if (name === 'proof_reports' && approved) {
      eq(entry.availability, 'available', 'capability.proof_reports');
      eq(entry.path, '/api/v1/proof-summary', 'capability.proof_reports.path');
    } else if (disabledNames.has(name)) {
      eq(entry.availability, 'not_enabled', `capability.${name}`);
      if (entry.path !== undefined) throw new Error(`Disabled capability has path: ${name}`);
    } else {
      throw new Error(`Unexpected capability: ${name}`);
    }
    return { name, availability: entry.availability as Capability['availability'], ...(entry.path === undefined ? {} : { path: entry.path as string }) };
  });
  if ([...availableNames, ...disabledNames].some((name) => !seen.has(name))) throw new Error('Missing known capabilities');
  return { schema_version: SCHEMA, mode: approved ? 'approved_detached_historical_report_read_only' : 'offline_contracts_only', capabilities, limitations: strings(v.limitations, 'capabilities.limitations') };
}

const scenarioIds = new Set(['FD-001', 'FD-002', 'FD-003', 'FD-004', 'FD-005']);
export function parseScenarios(value: unknown): Scenarios {
  const v = object(value, 'scenarios');
  version(v);
  eq(v.source, 'versioned_planning_contracts_only', 'scenarios.source');
  if (!Array.isArray(v.scenarios) || v.scenarios.length !== 5) throw new Error('scenarios: expected five planning contracts');
  const seen = new Set<string>();
  const scenarios = v.scenarios.map((item: unknown, i: number): Scenario => {
    const entry = object(item, `scenarios[${i}]`);
    const id = text(entry.id, 'scenario.id');
    if (!scenarioIds.has(id) || seen.has(id)) throw new Error(`Duplicate or unsupported scenario: ${id}`);
    seen.add(id);
    eq(entry.schema_version, '1.0.0', 'scenario.schema_version');
    eq(entry.contract_status, 'spec_only_not_implemented', 'scenario.contract_status');
    eq(entry.security_verdict, 'not_tested', 'scenario.security_verdict');
    eq(entry.evidence_loaded, false, 'scenario.evidence_loaded');
    const controls = strings(entry.mapped_controls, 'scenario.mapped_controls');
    if (controls.length === 0 || controls.some((c) => !/^C-\d{3}$/.test(c))) throw new Error('Invalid control reference');
    return {
      schema_version: '1.0.0', id, name: text(entry.name, 'scenario.name'), claim: text(entry.claim, 'scenario.claim'),
      mapped_controls: controls, contract_status: 'spec_only_not_implemented', security_verdict: 'not_tested', evidence_loaded: false,
    };
  });
  return { schema_version: SCHEMA, source: 'versioned_planning_contracts_only', scenarios, limitations: strings(v.limitations, 'scenarios.limitations') };
}

// The HTTP response is a minimized projection produced by the Go 6C1 verifier,
// not an independent attestation. Reject any attempt to claim current enforcement.
export const PROOF_SCHEMA = 'firedoor.v06.console-proof.v1' as const;
export interface ProofSummary {
  schema_version: typeof PROOF_SCHEMA; source_report_schema: 'firedoor.v05.proof-report.v1'; source_report_sha256: string;
  verdict: 'inconclusive'; evidence_status: 'historical_context_only_not_a_proof_chain';
  proof_chain_established: false; independently_attested: false; local_consistency_only: true;
  terraform: { receipt_id: string; captured_utc: string; source_sha256: string; add: number; change: number; remove: number; applied: false; evidence_type: 'speculative_plan_not_apply' };
  kubernetes: { cluster: 'firedoor-local'; context: 'kind-firedoor-local'; profile: 'hardened'; captured_utc: string; namespace_count: 3; ready_pod_count: 3; policy_count: 2; evidence_type: 'read_only_snapshot_at_collection_time'; current_read: false };
  historical_witnesses: Array<{route_id: string; captured_utc: string; historical_only: true; restoration_verified: true; phases: Array<{phase: string; observed: 'unreachable' | 'reachable'}>}>;
  limitations: string[];
}
function hash(value: unknown, label: string): string {
  const result = text(value, label);
  if (!/^[a-f0-9]{64}$/.test(result)) throw new Error(`${label}: invalid SHA-256`);
  return result;
}
function count(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0 || value > 100000) throw new Error(`${label}: invalid count`);
  return value;
}
function utc(value: unknown, label: string): string {
  const result = text(value, label);
  if (!/^\d{4}-\d{2}-\d{2}T.*(?:Z|[+-]\d{2}:\d{2})$/.test(result) || Number.isNaN(Date.parse(result))) throw new Error(`${label}: invalid UTC timestamp`);
  return result;
}
export function parseProofSummary(value: unknown): ProofSummary {
  const v = object(value, 'proof_summary');
  exactFields(v, ['schema_version', 'source_report_schema', 'source_report_sha256', 'verdict', 'evidence_status',
    'proof_chain_established', 'independently_attested', 'local_consistency_only', 'terraform', 'kubernetes',
    'historical_witnesses', 'limitations'], 'proof_summary');
  eq(v.schema_version, PROOF_SCHEMA, 'proof_summary.schema_version');
  eq(v.source_report_schema, 'firedoor.v05.proof-report.v1', 'proof_summary.source_report_schema');
  eq(v.verdict, 'inconclusive', 'proof_summary.verdict');
  eq(v.evidence_status, 'historical_context_only_not_a_proof_chain', 'proof_summary.evidence_status');
  eq(v.proof_chain_established, false, 'proof_summary.proof_chain_established');
  eq(v.independently_attested, false, 'proof_summary.independently_attested');
  eq(v.local_consistency_only, true, 'proof_summary.local_consistency_only');
  const plan = object(v.terraform, 'terraform');
  exactFields(plan, ['receipt_id', 'captured_utc', 'source_sha256', 'add', 'change', 'remove', 'applied', 'evidence_type'], 'terraform');
  eq(plan.applied, false, 'terraform.applied');
  eq(plan.evidence_type, 'speculative_plan_not_apply', 'terraform.evidence_type');
  const kube = object(v.kubernetes, 'kubernetes');
  exactFields(kube, ['cluster', 'context', 'profile', 'captured_utc', 'namespace_count', 'ready_pod_count',
    'policy_count', 'evidence_type', 'current_read'], 'kubernetes');
  eq(kube.cluster, 'firedoor-local', 'kubernetes.cluster');
  eq(kube.context, 'kind-firedoor-local', 'kubernetes.context');
  eq(kube.profile, 'hardened', 'kubernetes.profile');
  eq(kube.evidence_type, 'read_only_snapshot_at_collection_time', 'kubernetes.evidence_type');
  eq(kube.current_read, false, 'kubernetes.current_read');
  eq(kube.namespace_count, 3, 'kubernetes.namespace_count');
  eq(kube.ready_pod_count, 3, 'kubernetes.ready_pod_count');
  eq(kube.policy_count, 2, 'kubernetes.policy_count');
  if (!Array.isArray(v.historical_witnesses) || v.historical_witnesses.length !== 2) throw new Error('historical_witnesses: expected two historical records');
  const expectedRoutes = ['data-to-app', 'edge-to-data'];
  const witnesses: ProofSummary['historical_witnesses'] = v.historical_witnesses.map((raw: unknown, i: number) => {
    const w = object(raw, `witness[${i}]`);
    exactFields(w, ['route_id', 'captured_utc', 'historical_only', 'restoration_verified', 'phases'], `witness[${i}]`);
    eq(w.route_id, expectedRoutes[i], `witness[${i}].route_id`);
    eq(w.historical_only, true, `witness[${i}].historical_only`);
    eq(w.restoration_verified, true, `witness[${i}].restoration_verified`);
    if (!Array.isArray(w.phases) || w.phases.length !== 3) throw new Error('witness phases: invalid sequence');
    const phases = w.phases.map((rawPhase: unknown, j: number) => {
      const phase = object(rawPhase, `phase[${j}]`);
      exactFields(phase, ['phase', 'observed'], `phase[${j}]`);
      eq(phase.phase, ['before', 'witness_present', 'restored'][j], `phase[${j}].phase`);
      const observed = j === 1 ? 'reachable' : 'unreachable';
      eq(phase.observed, observed, `phase[${j}].observed`);
      return { phase: phase.phase as string, observed } as const;
    });
    return { route_id: expectedRoutes[i], captured_utc: utc(w.captured_utc, 'witness.captured_utc'), historical_only: true, restoration_verified: true, phases };
  });
  const limitations = strings(v.limitations, 'proof_summary.limitations');
  const expectedLimitations = [
    'A speculative Terraform plan proposes changes; it does not apply infrastructure or establish Kubernetes enforcement.',
    'A fresh Kubernetes inventory is a configuration snapshot, not a live network traffic test.',
    'Historical v0.4 A/B/A HTTP observations predate this plan and are NOT contemporaneous or proof of a single causal chain.',
    'Restoration and journal checks are local record validations; no historical witness is replayed.',
    'SHA256 values only detect local inconsistency and are not signatures, remote attestations, or independently authenticated evidence.',
    'The overall Firedoor Proof Lens security verdict is inconclusive.',
  ];
  if (limitations.length !== expectedLimitations.length || limitations.some((entry, i) => entry !== expectedLimitations[i])) {
    throw new Error('Published historical limitations were modified');
  }
  const planTime = utc(plan.captured_utc, 'plan.captured_utc');
  const observationTime = utc(kube.captured_utc, 'kubernetes.captured_utc');
  if (Date.parse(observationTime) < Date.parse(planTime) ||
    witnesses.some((w) => Date.parse(w.captured_utc) >= Date.parse(planTime))) {
    throw new Error('Historical proof chronology is invalid');
  }
  const receiptID = text(plan.receipt_id, 'terraform.receipt_id');
  if (!/^TFPLAN-[A-Za-z0-9-]{8,121}$/.test(receiptID)) throw new Error('Invalid plan receipt ID');
  return {
    schema_version: PROOF_SCHEMA, source_report_schema: 'firedoor.v05.proof-report.v1', source_report_sha256: hash(v.source_report_sha256, 'source_report_sha256'),
    verdict: 'inconclusive', evidence_status: 'historical_context_only_not_a_proof_chain',
    proof_chain_established: false, independently_attested: false, local_consistency_only: true,
    terraform: { receipt_id: receiptID, captured_utc: planTime, source_sha256: hash(plan.source_sha256, 'terraform.source_sha256'),
      add: count(plan.add, 'terraform.add'), change: count(plan.change, 'terraform.change'), remove: count(plan.remove, 'terraform.remove'), applied: false, evidence_type: 'speculative_plan_not_apply' },
    kubernetes: { cluster: 'firedoor-local', context: 'kind-firedoor-local', profile: 'hardened', captured_utc: observationTime,
      namespace_count: 3, ready_pod_count: 3, policy_count: 2, evidence_type: 'read_only_snapshot_at_collection_time', current_read: false },
    historical_witnesses: witnesses, limitations,
  };
}


// 6D2: Read-only v0.6 projections, never evaluated control findings or live inventory.
export interface ControlDefinition {
  id: string;
  legacy_id: string;
  name: string;
  domain: 'identity' | 'network' | 'workload';
  severity: 'info' | 'low' | 'medium' | 'high' | 'critical';
  description: string;
}
export interface ControlCatalog {
  schema_version: typeof SCHEMA;
  source: 'go_control_definitions_only';
  evaluation_status: 'not_evaluated';
  definitions: ControlDefinition[];
  limitations: string[];
}
export interface HistoricalEnvironment {
  schema_version: typeof SCHEMA;
  source: 'approved_detached_historical_report_only';
  observation_status: 'historical_snapshot_not_current';
  cluster: 'firedoor-local';
  context: 'kind-firedoor-local';
  profile: 'hardened';
  captured_utc: string;
  namespace_count: number;
  ready_pod_count: number;
  policy_count: number;
  current_read: false;
  enforcement_verified: false;
  limitations: string[];
}

const controlIDs = ['C-001', 'C-002', 'C-003', 'C-004', 'C-006', 'C-007'];
export function parseControlCatalog(value: unknown): ControlCatalog {
  const v = object(value, 'control_catalog');
  exactFields(v, ['schema_version', 'source', 'evaluation_status', 'definitions', 'limitations'], 'control_catalog');
  version(v);
  eq(v.source, 'go_control_definitions_only', 'control_catalog.source');
  eq(v.evaluation_status, 'not_evaluated', 'control_catalog.evaluation_status');
  if (!Array.isArray(v.definitions) || v.definitions.length !== 6) throw new Error('control_catalog: expected six definitions');
  const seen = new Set<string>();
  const definitions = v.definitions.map((raw: unknown, index: number): ControlDefinition => {
    const d = object(raw, `definition[${index}]`);
    exactFields(d, ['id', 'legacy_id', 'name', 'domain', 'severity', 'description'], `definition[${index}]`);
    const legacy_id = text(d.legacy_id, 'definition.legacy_id');
    if (!controlIDs.includes(legacy_id) || seen.has(legacy_id)) throw new Error('Unknown or duplicated control ID');
    seen.add(legacy_id);
    const id = text(d.id, 'definition.id');
    if (!/^CTRL-(ID|NET|WL)-\d{3}$/.test(id)) throw new Error('Invalid canonical control ID');
    const domain = text(d.domain, 'definition.domain');
    const severity = text(d.severity, 'definition.severity');
    if (!['identity', 'network', 'workload'].includes(domain)) throw new Error('Unsupported control domain');
    if (!['info', 'low', 'medium', 'high', 'critical'].includes(severity)) throw new Error('Unsupported control severity');
    return { id, legacy_id, name: text(d.name, 'definition.name'), domain: domain as ControlDefinition['domain'],
      severity: severity as ControlDefinition['severity'], description: text(d.description, 'definition.description') };
  });
  if (controlIDs.some((id) => !seen.has(id))) throw new Error('Control catalog missing a foundation definition');
  const limitations = strings(v.limitations, 'control_catalog.limitations');
  if (limitations.length !== 2 || !limitations[0].includes('specifications') || !limitations[1].includes('No Kubernetes inspection')) {
    throw new Error('Control catalog limitations absent or misleading');
  }
  return { schema_version: SCHEMA, source: 'go_control_definitions_only', evaluation_status: 'not_evaluated', definitions, limitations };
}

export function parseHistoricalEnvironment(value: unknown): HistoricalEnvironment {
  const v = object(value, 'historical_environment');
  exactFields(v, ['schema_version', 'source', 'observation_status', 'cluster', 'context', 'profile', 'captured_utc',
    'namespace_count', 'ready_pod_count', 'policy_count', 'current_read', 'enforcement_verified', 'limitations'], 'historical_environment');
  version(v);
  eq(v.source, 'approved_detached_historical_report_only', 'historical_environment.source');
  eq(v.observation_status, 'historical_snapshot_not_current', 'historical_environment.observation_status');
  eq(v.cluster, 'firedoor-local', 'historical_environment.cluster');
  eq(v.context, 'kind-firedoor-local', 'historical_environment.context');
  eq(v.profile, 'hardened', 'historical_environment.profile');
  eq(v.current_read, false, 'historical_environment.current_read');
  eq(v.enforcement_verified, false, 'historical_environment.enforcement_verified');
  const limitations = strings(v.limitations, 'historical_environment.limitations');
  if (limitations.length !== 3 || !limitations[0].includes('not a current') || !limitations[1].includes('do not establish enforcement') || !limitations[2].includes('not an independently authenticated')) {
    throw new Error('Historical environment limitations absent or misleading');
  }
  return { schema_version: SCHEMA, source: 'approved_detached_historical_report_only',
    observation_status: 'historical_snapshot_not_current', cluster: 'firedoor-local', context: 'kind-firedoor-local', profile: 'hardened',
    captured_utc: utc(v.captured_utc, 'historical_environment.captured_utc'),
    namespace_count: count(v.namespace_count, 'historical_environment.namespace_count'),
    ready_pod_count: count(v.ready_pod_count, 'historical_environment.ready_pod_count'),
    policy_count: count(v.policy_count, 'historical_environment.policy_count'), current_read: false, enforcement_verified: false, limitations };
}
