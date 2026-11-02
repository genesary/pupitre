import { describe, expect, it } from 'vitest';

import { nodePosition, roadPath, viewBox, wrapLabel } from '@/lib/path-map';

describe('path map', () => {
  it('snakes back on the second row', () => {
    expect(nodePosition(0)).toEqual({ x: 40, y: 45 });
    expect(nodePosition(4)).toEqual({ x: 840, y: 45 });
    expect(nodePosition(5)).toEqual({ x: 840, y: 170 });
    expect(nodePosition(6)).toEqual({ x: 640, y: 170 });
  });

  it('sizes the view box to the steps', () => {
    expect(viewBox(3)).toEqual({ width: 480, height: 121 });
    expect(viewBox(6).height).toBe(246);
  });

  it('draws straight segments and rounds the turns', () => {
    expect(roadPath([nodePosition(0)])).toBe('');
    expect(roadPath([nodePosition(0), nodePosition(1), nodePosition(2)])).toBe('M 40 45 L 240 45 L 440 45');
    expect(roadPath([nodePosition(3), nodePosition(4), nodePosition(5)])).toContain(' Q 840 45 ');
  });

  it('wraps long labels on a space', () => {
    expect(wrapLabel('Linux', 16)).toEqual(['Linux']);
    expect(wrapLabel('Networking Basics Advanced', 16)).toEqual(['Networking', 'Basics Advanced']);
  });
});
