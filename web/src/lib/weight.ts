import type { RecordModel } from 'pocketbase';
import { addDays } from './diary';

// Weigh-ins and what the weight screen works out from them: a smoothed trend
// that day-to-day water weight doesn't throw around, the recent weekly rate,
// and when the goal is reached at that rate.

export interface Weighin extends RecordModel {
  user: string;
  date: string;
  weight_kg: number;
  note: string;
}

export interface Point {
  date: string;
  kg: number;
  /** the smoothed weight on that day */
  trend: number;
}

const DAY_MS = 86_400_000;

/** Days from a to b, both YYYY-MM-DD. */
export function daysBetween(a: string, b: string): number {
  return Math.round((Date.parse(b + 'T12:00:00Z') - Date.parse(a + 'T12:00:00Z')) / DAY_MS);
}

/**
 * The trend: an exponential moving average that moves a tenth of the way to
 * each day's weight, compounded over the days between weigh-ins.
 */
export function withTrend(list: Pick<Weighin, 'date' | 'weight_kg'>[]): Point[] {
  const sorted = [...list].sort((a, b) => a.date.localeCompare(b.date));
  const points: Point[] = [];
  for (const w of sorted) {
    const prev = points.at(-1);
    let trend = w.weight_kg;
    if (prev) {
      const keep = Math.pow(0.9, Math.max(daysBetween(prev.date, w.date), 1));
      trend = prev.trend * keep + w.weight_kg * (1 - keep);
    }
    points.push({ date: w.date, kg: w.weight_kg, trend });
  }
  return points;
}

/**
 * The change in kg a week over the last `days` days, from a straight line
 * through the weigh-ins; null until there are three spanning a week.
 */
export function weeklyRate(points: Point[], today: string, days = 28): number | null {
  const recent = points.filter((p) => daysBetween(p.date, today) <= days);
  if (recent.length < 3 || daysBetween(recent[0].date, recent.at(-1)!.date) < 7) return null;
  const xs = recent.map((p) => daysBetween(recent[0].date, p.date));
  const ys = recent.map((p) => p.kg);
  const mx = xs.reduce((s, x) => s + x, 0) / xs.length;
  const my = ys.reduce((s, y) => s + y, 0) / ys.length;
  let num = 0;
  let den = 0;
  for (let i = 0; i < xs.length; i++) {
    num += (xs[i] - mx) * (ys[i] - my);
    den += (xs[i] - mx) ** 2;
  }
  return (num / den) * 7;
}

/**
 * When the trend reaches the goal at a weekly rate, YYYY-MM-DD; null when it
 * is heading the other way or would take more than two years.
 */
export function projectedDate(
  from: number,
  goal: number,
  rate: number | null,
  today: string,
): string | null {
  if (!rate || Math.sign(goal - from) !== Math.sign(rate)) return null;
  const days = Math.ceil(((goal - from) / rate) * 7);
  return days > 730 ? null : addDays(today, days);
}

/** How far along from the start weight to the goal, 0 to 1. */
export function progress(start: number, now: number, goal: number): number {
  if (start === goal) return 1;
  return Math.min(Math.max((start - now) / (start - goal), 0), 1);
}

export const ranges = [
  { id: '1m', label: '1M', days: 30 },
  { id: '3m', label: '3M', days: 91 },
  { id: '6m', label: '6M', days: 182 },
  { id: '1y', label: '1Y', days: 365 },
  { id: 'all', label: 'All', days: Infinity },
] as const;
export type Range = (typeof ranges)[number]['id'];

/** About `count` round values covering min to max, for the chart's axis. */
export function ticks(min: number, max: number, count = 4): number[] {
  const span = Math.max(max - min, 0.5);
  const raw = span / count;
  const mag = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((s) => s >= raw)!;
  const out: number[] = [];
  for (let v = Math.floor(min / step) * step; v <= max + step / 2; v += step) {
    out.push(Math.round(v * 100) / 100);
  }
  return out;
}

/** A signed change to show, such as "−1.2 kg" or "+0.4 lb". */
export function formatChange(kg: number, units: 'metric' | 'imperial'): string {
  const v = units === 'imperial' ? kg / 0.45359237 : kg;
  const r = Math.round(v * 10) / 10;
  const sign = r > 0 ? '+' : r < 0 ? '−' : '±';
  return `${sign}${Math.abs(r)} ${units === 'imperial' ? 'lb' : 'kg'}`;
}

/** A short date such as "12 Mar", with the year when it isn't this year's. */
export function shortDate(day: string, today: string): string {
  const d = new Date(day + 'T12:00:00');
  return d.toLocaleDateString(undefined, {
    day: 'numeric',
    month: 'short',
    year: day.slice(0, 4) === today.slice(0, 4) ? undefined : 'numeric',
  });
}
