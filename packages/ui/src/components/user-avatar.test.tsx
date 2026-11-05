import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { UserAvatar } from './user-avatar';

// Audit finding C4: avatarUrl is free text on the server.
describe('UserAvatar', () => {
  it('renders an https image without referrer', () => {
    const { container } = render(<UserAvatar username="alice" avatarUrl="https://cdn.example.com/a.png" />);
    const img = container.querySelector('img');
    expect(img).toHaveAttribute('src', 'https://cdn.example.com/a.png');
    expect(img).toHaveAttribute('referrerpolicy', 'no-referrer');
  });

  it('falls back to the initial when the image fails', () => {
    const { container } = render(<UserAvatar username="alice" avatarUrl="https://cdn.example.com/a.png" />);
    fireEvent.error(container.querySelector('img')!);
    expect(screen.getByText('A')).toBeInTheDocument();
  });

  it('renders an emoji as text', () => {
    render(<UserAvatar username="alice" avatarUrl="🦊" />);
    expect(screen.getByText('🦊')).toBeInTheDocument();
  });

  it('never parses an HTML payload', () => {
    const { container } = render(
      <UserAvatar username="mallory" avatarUrl={'<img src=x onerror=alert(1)>'} />,
    );
    expect(container.querySelector('img')).toBeNull();
    expect(container.textContent).toBe('M');
  });

  it('rejects an attribute-breaking URL instead of rendering it', () => {
    // The C4 payload of the audit: not a valid URL, so no <img> at all.
    const payload = 'https://x" onerror="alert(1)';
    const { container } = render(<UserAvatar username="mallory" avatarUrl={payload} />);
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('[onerror]')).toBeNull();
    expect(container.textContent).toBe('M');
  });

  it('ignores javascript: URLs', () => {
    const { container } = render(<UserAvatar username="mallory" avatarUrl="javascript:alert(1)" />);
    expect(container.querySelector('img')).toBeNull();
  });

  it('renders a username payload as text', () => {
    render(<UserAvatar username={'<script>x</script>'} avatarUrl={null} />);
    expect(screen.getByText('<')).toBeInTheDocument();
  });
});
