import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { TourBubble } from './TourBubble';

describe('tour bubble (spec 007)', () => {
  it('keeps Shift+Tab inside while a step waits and the bubble holds focus', () => {
    render(
      <TourBubble rect={null} stepKey="waiting" title="…" actions={<button>Passer la visite</button>}>
        {null}
      </TourBubble>,
    );
    const bubble = screen.getByRole('dialog');
    expect(bubble).toHaveFocus();

    fireEvent.keyDown(bubble, { key: 'Tab', shiftKey: true });
    expect(screen.getByRole('button', { name: 'Passer la visite' })).toHaveFocus();
  });

  it('labels the dialog with its own title and text', () => {
    render(
      <TourBubble rect={null} stepKey="one" title="Le catalogue" actions={null}>
        Tous les cours
      </TourBubble>,
    );
    expect(screen.getByRole('dialog', { name: 'Le catalogue' })).toHaveAccessibleDescription('Tous les cours');
  });
});
