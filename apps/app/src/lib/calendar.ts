// Month grid of the agenda (docs/specs/022-agenda-and-sessions.md), as built by
// pupitre's agenda.astro: weeks run Monday to Sunday, padded with the days of
// the previous and next months.

/** Local day key, YYYY-MM-DD. */
export function dayKey(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

export type CalendarCell = { key: string; day: number; otherMonth: boolean };

export function monthGrid(year: number, month: number): CalendarCell[] {
  const first = new Date(year, month, 1).getDay();
  const offset = first === 0 ? 6 : first - 1;
  const days = new Date(year, month + 1, 0).getDate();
  const total = Math.ceil((offset + days) / 7) * 7;
  return Array.from({ length: total }, (_, i) => {
    const date = new Date(year, month, i - offset + 1);
    return { key: dayKey(date), day: date.getDate(), otherMonth: date.getMonth() !== month };
  });
}

export const MONTHS_SHORT = ['Jan', 'Fév', 'Mar', 'Avr', 'Mai', 'Juin', 'Juil', 'Aoû', 'Sep', 'Oct', 'Nov', 'Déc'];

/** ISO date → value of an <input type="datetime-local">, in local time. */
export function toLocalInput(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${dayKey(date)}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}
