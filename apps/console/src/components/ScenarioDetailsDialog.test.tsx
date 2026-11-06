import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import ScenarioDetailsDialog, { dialogKeyboardAction } from './ScenarioDetailsDialog';
import type { Scenario } from '../api/contracts';

const contract: Scenario = {
  schema_version: '1.0.0', id: 'FD-001', name: 'Compromised Workload',
  claim: 'A planning statement, not an observed outcome.', mapped_controls: ['C-004'],
  contract_status: 'spec_only_not_implemented', security_verdict: 'not_tested', evidence_loaded: false,
};

describe('6E1A scenario drawer accessibility', () => {
  it('renders a named, described modal dialog with keyboard-accessible close controls', () => {
    const html = renderToStaticMarkup(<ScenarioDetailsDialog scenario={contract} onClose={() => undefined}/>);
    expect(html).toContain('role="dialog"');
    expect(html).toContain('aria-modal="true"');
    expect(html).toContain('aria-labelledby="scenario-drawer-title"');
    expect(html).toContain('aria-describedby="scenario-drawer-description"');
    expect(html).toContain('aria-label="Close scenario details"');
    expect(html).toContain('Return to scenarios');
    expect(html).toContain('Not tested');
    expect(html).not.toContain('Enforcement confirmed');
  });

  it('closes on Escape regardless of which dialog button has focus', () => {
    expect(dialogKeyboardAction('Escape', false, 0, 2)).toBe('close');
    expect(dialogKeyboardAction('Escape', true, 1, 2)).toBe('close');
  });

  it('wraps forward Tab from the last focusable element to the first', () => {
    expect(dialogKeyboardAction('Tab', false, 1, 2)).toBe('focus-first');
    expect(dialogKeyboardAction('Tab', false, -1, 2)).toBe('focus-first');
    expect(dialogKeyboardAction('Tab', false, 0, 2)).toBe('none');
  });

  it('wraps Shift+Tab from the first focusable element to the last', () => {
    expect(dialogKeyboardAction('Tab', true, 0, 2)).toBe('focus-last');
    expect(dialogKeyboardAction('Tab', true, -1, 2)).toBe('focus-last');
    expect(dialogKeyboardAction('Tab', true, 1, 2)).toBe('none');
  });

  it('does not interfere with unrelated keys or a dialog without focusable elements', () => {
    expect(dialogKeyboardAction('Enter', false, 1, 2)).toBe('none');
    expect(dialogKeyboardAction('Tab', false, -1, 0)).toBe('none');
  });
});
