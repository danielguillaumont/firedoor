import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import ScenarioEvidenceReview from './ScenarioEvidenceReview';

describe('H3 historical scenario evidence UI boundary', () => {
  it('starts with no evidence and does not certify FD-004', () => {
    const html = renderToStaticMarkup(<ScenarioEvidenceReview />);
    expect(html).toContain('No historical result loaded');
    expect(html).toContain('Not tested');
    expect(html).toContain('type="file"');
    expect(html).toContain('Select H2 scenario result');
    expect(html).not.toContain('Scenario passed');
    expect(html).not.toContain('Current cluster verified');
  });
});
