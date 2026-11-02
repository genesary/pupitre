// Geometry of the progress map of a learning path, from pupitre's my-paths.astro:
// the steps snake across rows of five, joined by a road with rounded turns.

export type Point = { x: number; y: number };

export const MAP = {
  perRow: 5,
  nodeRadius: 18,
  columnGap: 200,
  rowGap: 125,
  road: 6,
  cornerRadius: 28,
  labelHeight: 38,
  paddingX: 40,
  paddingY: 45,
} as const;

/** Position of step i: left to right on even rows, right to left on odd rows. */
export function nodePosition(index: number): Point {
  const row = Math.floor(index / MAP.perRow);
  const column = index % MAP.perRow;
  const xIndex = row % 2 === 0 ? column : MAP.perRow - 1 - column;
  return { x: MAP.paddingX + xIndex * MAP.columnGap, y: MAP.paddingY + row * MAP.rowGap };
}

export function viewBox(count: number): { width: number; height: number } {
  const rows = Math.ceil(count / MAP.perRow);
  const columns = Math.min(count, MAP.perRow);
  return {
    width: MAP.paddingX * 2 + MAP.columnGap * Math.max(columns - 1, 0),
    height: MAP.paddingY + MAP.rowGap * Math.max(rows - 1, 0) + MAP.nodeRadius + MAP.labelHeight + 20,
  };
}

/** SVG path through the points, with a quadratic curve at each turn. */
export function roadPath(points: Point[], radius: number = MAP.cornerRadius): string {
  if (points.length < 2) return '';
  let d = `M ${points[0]!.x} ${points[0]!.y}`;
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1]!;
    const curr = points[i]!;
    const next = points[i + 1];
    const straight =
      !next || (prev.x === curr.x && curr.x === next.x) || (prev.y === curr.y && curr.y === next.y);
    if (straight) {
      d += ` L ${curr.x} ${curr.y}`;
      continue;
    }
    const dx1 = curr.x - prev.x;
    const dy1 = curr.y - prev.y;
    const dx2 = next.x - curr.x;
    const dy2 = next.y - curr.y;
    const t = Math.min(radius / Math.hypot(dx1, dy1), 0.42);
    const s = Math.min(radius / Math.hypot(dx2, dy2), 0.42);
    d += ` L ${curr.x - dx1 * t} ${curr.y - dy1 * t} Q ${curr.x} ${curr.y} ${curr.x + dx2 * s} ${curr.y + dy2 * s}`;
  }
  return d;
}

/** Splits a label in two lines at most, on a space when possible. */
export function wrapLabel(text: string, max: number): string[] {
  if (text.length <= max) return [text];
  const space = text.lastIndexOf(' ', max);
  return space > 0 ? [text.slice(0, space), text.slice(space + 1)] : [text.slice(0, max), text.slice(max)];
}
