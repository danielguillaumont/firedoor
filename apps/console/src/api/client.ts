import { parseCapabilities, parseHealth, parseScenarios, parseProofSummary, parseControlCatalog, parseHistoricalEnvironment } from './contracts';

export const endpoints = {
  health: '/api/v1/health',
  capabilities: '/api/v1/capabilities',
  scenarios: '/api/v1/scenarios',
  proof: '/api/v1/proof-summary',
  controls: '/api/v1/control-catalog',
  environment: '/api/v1/environment-snapshot',
} as const;

// Bound the opt-in proof response even when the server sends chunked content
// without Content-Length. A report larger than this is not our minimized API.
async function readBoundedProof(response: Response): Promise<unknown> {
  const maxBytes = 32 * 1024;
  const declared = response.headers.get('Content-Length');
  if (declared && Number(declared) > maxBytes) throw new Error('Approved proof projection exceeds size limit');
  const reader = response.body?.getReader();
  if (!reader) throw new Error('Approved proof response has no body');
  const decoder = new TextDecoder('utf-8', { fatal: true });
  let size = 0;
  let json = '';
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > maxBytes) throw new Error('Approved proof projection exceeds size limit');
      json += decoder.decode(value, { stream: true });
    }
    json += decoder.decode();
    return JSON.parse(json) as unknown;
  } catch (error) {
    await reader.cancel().catch(() => undefined);
    throw error;
  } finally {
    reader.releaseLock();
  }
}

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
  return parser((path === endpoints.proof || path === endpoints.controls || path === endpoints.environment)
    ? await readBoundedProof(response) : await response.json() as unknown);
}

export const loadHealth = (signal: AbortSignal) => getJSON(endpoints.health, parseHealth, signal);
export const loadCapabilities = (signal: AbortSignal) => getJSON(endpoints.capabilities, parseCapabilities, signal);
export const loadScenarios = (signal: AbortSignal) => getJSON(endpoints.scenarios, parseScenarios, signal);

// Only called following deliberate user interaction and advertised server capability.
export const loadApprovedProofSummary = (signal: AbortSignal) => getJSON(endpoints.proof, parseProofSummary, signal);

// Catalog reads only static definitions. The environment loader is called solely after user action.
export const loadControlCatalog = (signal: AbortSignal) => getJSON(endpoints.controls, parseControlCatalog, signal);
export const loadHistoricalEnvironment = (signal: AbortSignal) => getJSON(endpoints.environment, parseHistoricalEnvironment, signal);
