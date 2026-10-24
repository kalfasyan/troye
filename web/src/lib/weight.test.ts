import { describe, expect, it } from 'vitest';
import {
  daysBetween,
  formatChange,
  progress,
  projectedDate,
  ticks,
  weeklyRate,
  withTrend,
} from './weight';

const w = (date: string, weight_kg: number) => ({ date, weight_kg });

describe('trend', () => {
  it('moves a tenth of the way each day', () => {
    const p = withTrend([w('2026-10-02', 79), w('2026-10-01', 80)]);
    expect(p.map((x) => x.date)).toEqual(['2026-10-01', '2026-10-02']);
    expect(p[0].trend).toBe(80);
    expect(p[1].trend).toBeCloseTo(79.9);
  });

  it('compounds over the days between weigh-ins', () => {
    const p = withTrend([w('2026-10-01', 80), w('2026-10-08', 78)]);
    // 1 - 0.9^7 of the way, about 52%
    expect(p[1].trend).toBeCloseTo(80 - 2 * (1 - 0.9 ** 7));
  });
});

describe('rate and projection', () => {
  const steady = withTrend(
    Array.from({ length: 15 }, (_, i) =>
      w(`2026-10-${String(i * 2 + 1).padStart(2, '0')}`, 80 - i * 0.1),
    ),
  );

  it('fits the recent weigh-ins', () => {
    // 0.1 kg every 2 days is 0.35 kg a week
    expect(weeklyRate(steady, '2026-10-29')).toBeCloseTo(-0.35);
  });

  it('needs three weigh-ins over a week', () => {
    expect(weeklyRate(steady.slice(0, 2), '2026-10-03')).toBeNull();
    expect(weeklyRate(steady.slice(0, 3), '2026-10-05')).toBeNull();
  });

  it('projects the goal date only when heading there', () => {
    expect(daysBetween('2026-10-24', '2026-11-07')).toBe(14);
    expect(projectedDate(78, 77, -0.5, '2026-10-24')).toBe('2026-11-07');
    expect(projectedDate(78, 77, 0.2, '2026-10-24')).toBeNull();
    expect(projectedDate(78, 77, null, '2026-10-24')).toBeNull();
    expect(projectedDate(90, 70, -0.1, '2026-10-24')).toBeNull();
  });
});

describe('display', () => {
  it('measures progress either way', () => {
    expect(progress(80, 75, 70)).toBe(0.5);
    expect(progress(60, 63, 66)).toBe(0.5);
    expect(progress(80, 82, 70)).toBe(0);
  });

  it('makes round ticks', () => {
    expect(ticks(76.3, 80.2)).toEqual([76, 77, 78, 79, 80]);
    expect(ticks(70, 90)).toEqual([70, 75, 80, 85, 90]);
  });

  it('signs changes', () => {
    expect(formatChange(-1.24, 'metric')).toBe('−1.2 kg');
    expect(formatChange(1, 'imperial')).toBe('+2.2 lb');
    expect(formatChange(0.01, 'metric')).toBe('±0 kg');
  });
});
