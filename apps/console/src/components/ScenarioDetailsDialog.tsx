import { useEffect, useRef } from 'react';
import type { KeyboardEvent } from 'react';
import type { Scenario } from '../api/contracts';
import Icon from './Icon';
import ScenarioEvidenceReview from './ScenarioEvidenceReview';

// Keep keyboard behavior testable without simulating an actual browser or
// weakening Firedoor's node-only Vitest setup.
export type DialogKeyboardAction = 'close' | 'focus-first' | 'focus-last' | 'none';

export function dialogKeyboardAction(key: string, shift: boolean, activeIndex: number, count: number): DialogKeyboardAction {
  if (key === 'Escape') return 'close';
  if (key !== 'Tab' || count < 1) return 'none';
  if (shift && activeIndex <= 0) return 'focus-last';
  if (!shift && (activeIndex === count - 1 || activeIndex < 0)) return 'focus-first';
  return 'none';
}

export default function ScenarioDetailsDialog({ scenario, onClose, onViewProof }: { scenario: Scenario; onClose: () => void; onViewProof?: () => void }) {
  const dialog = useRef<HTMLElement>(null);
  const closeButton = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    closeButton.current?.focus();
    return () => {
      document.body.style.overflow = previousOverflow;
      if (previousFocus?.isConnected) previousFocus.focus();
    };
  }, []);

  const onDialogKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    const focusables = Array.from(dialog.current?.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    ) ?? []);
    const action = dialogKeyboardAction(event.key, event.shiftKey, focusables.indexOf(document.activeElement as HTMLElement), focusables.length);
    if (action === 'close') {
      event.preventDefault();
      event.stopPropagation();
      onClose();
    } else if (action === 'focus-first' || action === 'focus-last') {
      event.preventDefault();
      focusables[action === 'focus-first' ? 0 : focusables.length - 1].focus();
    }
  };

  return <div className="drawer-backdrop" role="presentation" onMouseDown={onClose}>
    <section ref={dialog} className="scenario-drawer" role="dialog" aria-modal="true"
      aria-labelledby="scenario-drawer-title" aria-describedby="scenario-drawer-description"
      onMouseDown={(event) => event.stopPropagation()} onKeyDown={onDialogKeyDown}>
      <div className="drawer-actions"><span className="scenario-id">{scenario.id} / CONTRACT</span>
        <button ref={closeButton} className="icon-button" type="button" aria-label="Close scenario details" onClick={onClose}>
          <Icon name="close" size={18}/>
        </button>
      </div>
      <div className="section-label"><span className="section-line"/>Security claim</div>
      <h2 id="scenario-drawer-title">{scenario.name}</h2>
      <p id="scenario-drawer-description" className="drawer-description">{scenario.claim}</p>
      <div className="drawer-property"><span>Execution status</span><strong>Not tested</strong></div>
      <div className="drawer-property"><span>Contract state</span><strong>Planning only</strong></div>
      <div className="drawer-property"><span>Scenario execution evidence</span><strong>No</strong></div>
      <div className="drawer-property"><span>Mapped controls</span><strong>{scenario.mapped_controls.join(', ')}</strong></div>
      <div className="drawer-limitation"><Icon name="info"/><span>This view describes the expected security claim. It is not evidence that a restriction was enforced.</span></div>
      {scenario.id === "FD-004" && <ScenarioEvidenceReview onViewProof={onViewProof}/>}
      <button type="button" className="button button-dark" onClick={onClose}>Return to scenarios <Icon name="arrow-left" size={16}/></button>
    </section>
  </div>;
}
