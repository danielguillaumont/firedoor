import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import App, { ScenarioCard } from './App';
import { parseScenarios, SCHEMA } from './api/contracts';

const fixture = parseScenarios({ schema_version: SCHEMA, source: 'versioned_planning_contracts_only', limitations: ['Nothing certified'], scenarios: [
  'Compromised Workload', 'Identity Overreach', 'Secret Boundary', 'Lateral Movement', 'Privileged Workload Admission',
].map((name, i) => ({ schema_version: '1.0.0', id: `FD-00${i + 1}`, name, claim: 'Bounded planning statement', mapped_controls: ['C-004'], contract_status: 'spec_only_not_implemented', security_verdict: 'not_tested', evidence_loaded: false })) });

describe('console UI evidence boundary', () => {
  it('renders a contract as not tested, without a synthetic successful verdict', () => {
    const html = renderToStaticMarkup(<ScenarioCard scenario={fixture.scenarios[3]} onOpen={() => undefined} />);
    expect(html).toContain('FD-004');
    expect(html).toContain('Not tested');
    expect(html).not.toContain('Contained');
    expect(html).not.toContain('Verified');
  });
  it('starts in a safe loading state without inventing scenario counts or lab health', () => {
    const html = renderToStaticMarkup(<App />);
    expect(html).toContain('Contract preview');
    expect(html).toContain('not a security assessment');
    expect(html).toContain('Connecting to API');
    expect(html).toContain('Skip to main content');
    expect(html).toContain('href="#firedoor-main"');
    expect(html).toContain('id="firedoor-main"');
    expect(html).toContain('History');
    expect(html).toContain('id="firedoor-navigation"');
    expect(html).toContain('aria-controls="firedoor-navigation"');
    expect(html).toContain('aria-expanded="false"');
    expect(html).toContain('Workspace navigation, current page Overview; open menu');
    expect(html).toContain('aria-current="page"');
    expect(html).not.toContain('Load historical activity');
    expect(html).toContain('not cluster health');
    expect(html).not.toContain('3 ready pods');
  });
});
