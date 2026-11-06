import { parseCapabilities, parseHealth, parseScenarios, parseProofSummary, parseControlCatalog, parseHistoricalEnvironment, parseHistoricalActivity } from './contracts';

export const endpoints = {
  health: '/api/v1/health',
  capabilities: '/api/v1/capabilities',
  scenarios: '/api/v1/scenarios',
  proof: '/api/v1/proof-summary',
  controls: '/api/v1/control-catalog',
  environment: '/api/v1/environment-snapshot',
  history: '/api/v1/historical-activity',
} as const;

// All seven local API responses have a small, reviewed JSON contract. A
// malformed or unexpectedly large response must not enter a React view.
// The deadline also keeps a nonresponding local service from hanging the UI.
const MAX_RESPONSE_BYTES = 32 * 1024;
const REQUEST_TIMEOUT_MS = 8_000;

async function readBoundedJSON(response: Response): Promise<unknown> {
  const declared = response.headers.get('Content-Length');
  if (declared !== null && (!/^\d+$/.test(declared) || !Number.isSafeInteger(Number(declared)) || Number(declared) > MAX_RESPONSE_BYTES)) {
    throw new Error('Local API response has invalid or excessive Content-Length');
  }

  const reader = response.body?.getReader();
  if (!reader) throw new Error('Local API JSON response has no body');
  const decoder = new TextDecoder('utf-8', { fatal: true });
  let received = 0;
  let json = '';
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      received += value.byteLength;
      if (received > MAX_RESPONSE_BYTES) throw new Error('Local API JSON response exceeds size limit');
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
  // Link the caller's page-navigation cancellation to an independent request
  // deadline; remove the listener and timer after either success or failure.
  const request = new AbortController();
  const cancel = () => request.abort();
  if (signal.aborted) request.abort();
  else signal.addEventListener('abort', cancel, { once: true });
  const deadline = setTimeout(cancel, REQUEST_TIMEOUT_MS);
  try {
    if (request.signal.aborted) throw new DOMException('API request cancelled', 'AbortError');
    const response = await fetch(path, {
      method: 'GET',
      headers: { Accept: 'application/json' },
      cache: 'no-store',
      credentials: 'omit',
      redirect: 'error',
      signal: request.signal,
    });
    if (!response.ok) throw new Error(`API returned HTTP ${response.status}`);
    // Check the media type itself, not a substring of an arbitrary header.
    const mediaType = (response.headers.get('Content-Type') ?? '').split(';', 1)[0].trim().toLowerCase();
    if (mediaType !== 'application/json') throw new Error('API did not return JSON');
    const data = await readBoundedJSON(response);
    if (request.signal.aborted) throw new DOMException('API request cancelled', 'AbortError');
    return parser(data);
  } finally {
    clearTimeout(deadline);
    signal.removeEventListener('abort', cancel);
  }
}

export const loadHealth = (signal: AbortSignal) => getJSON(endpoints.health, parseHealth, signal);
export const loadCapabilities = (signal: AbortSignal) => getJSON(endpoints.capabilities, parseCapabilities, signal);
export const loadScenarios = (signal: AbortSignal) => getJSON(endpoints.scenarios, parseScenarios, signal);

// Only called following deliberate user interaction and advertised server capability.
export const loadApprovedProofSummary = (signal: AbortSignal) => getJSON(endpoints.proof, parseProofSummary, signal);

// Catalog reads only static definitions. The environment loader is called solely after user action.
export const loadControlCatalog = (signal: AbortSignal) => getJSON(endpoints.controls, parseControlCatalog, signal);
export const loadHistoricalEnvironment = (signal: AbortSignal) => getJSON(endpoints.environment, parseHistoricalEnvironment, signal);

// No automatic access: the History page requests only after explicit user action.
export const loadHistoricalActivity = (signal: AbortSignal) => getJSON(endpoints.history, parseHistoricalActivity, signal);
