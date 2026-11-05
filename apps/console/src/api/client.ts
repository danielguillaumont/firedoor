import { parseCapabilities, parseHealth, parseScenarios, parseProofSummary } from './contracts';

export const endpoints = {
  health: '/api/v1/health',
  capabilities: '/api/v1/capabilities',
  scenarios: '/api/v1/scenarios',
  proof: '/api/v1/proof-summary',
} as const;

export async function getJSON<T>(path: (typeof endpoints)[keyof typeof endpoints], parser: (input: unknown) => T, signal: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    method: 'GET',
    headers: { Accept: 'application/json' },
    cache: 'no-store',
    credentials: 'omit',
    redirect: 'error',
    signal,
  });
  if (!response.ok) throw new Error(`API returned HTTP ${response.status}`);
  const contentType = response.headers.get('Content-Type') ?? '';
  if (!contentType.toLowerCase().includes('application/json')) throw new Error('API did not return JSON');
  return parser(await response.json() as unknown);
}

export const loadHealth = (signal: AbortSignal) => getJSON(endpoints.health, parseHealth, signal);
export const loadCapabilities = (signal: AbortSignal) => getJSON(endpoints.capabilities, parseCapabilities, signal);
export const loadScenarios = (signal: AbortSignal) => getJSON(endpoints.scenarios, parseScenarios, signal);

// Only called following deliberate user interaction and advertised server capability.
export const loadApprovedProofSummary = (signal: AbortSignal) => getJSON(endpoints.proof, parseProofSummary, signal);
