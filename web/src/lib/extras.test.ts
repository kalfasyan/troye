import { describe, expect, it } from 'vitest';
import { exerciseKcal, fastingState, formatDuration } from './extras';

const at = (s: string) => new Date(`2026-10-24T${s}:00`);

describe('exerciseKcal', () => {
  it('matches the server', () => {
    expect(exerciseKcal('dog_walk', 45, 80)).toBe(120);
    expect(exerciseKcal('run', 30, 80)).toBe(352);
    expect(exerciseKcal('other', 60, 0)).toBe(245);
  });
});

describe('fastingState', () => {
  it('is null without a fasting plan', () => {
    expect(fastingState(at('10:00'), '12:00', 'none')).toBeNull();
  });

  it('counts down to the window opening', () => {
    const s = fastingState(at('10:00'), '12:00', '16_8')!;
    expect(s.eating).toBe(false);
    expect(s.until).toEqual(at('12:00'));
    // fasting since 20:00 yesterday: 14 of 16 hours
    expect(s.progress).toBeCloseTo(14 / 16);
  });

  it('counts down to the window closing', () => {
    const s = fastingState(at('14:00'), '12:00', '16_8')!;
    expect(s.eating).toBe(true);
    expect(s.until).toEqual(at('20:00'));
    expect(s.progress).toBeCloseTo(2 / 8);
  });

  it('skips the days off', () => {
    // 2026-10-24 is a Saturday; the window closes at 20:00 and a fast would start
    expect(fastingState(at('14:00'), '12:00', '16_8', ['fri'])).toBeNull();
    expect(fastingState(at('14:00'), '12:00', '16_8', ['sat'])?.eating).toBe(true);
    // Friday's fast runs into Saturday morning
    expect(fastingState(at('10:00'), '12:00', '16_8', ['fri'])?.eating).toBe(false);
    expect(fastingState(at('10:00'), '12:00', '16_8', ['sat'])).toBeNull();
  });

  it('handles a window across midnight', () => {
    const s = fastingState(at('01:00'), '18:30', '16_8')!;
    expect(s.eating).toBe(true);
    expect(s.until).toEqual(at('02:30'));
    const after = fastingState(at('03:00'), '18:30', '16_8')!;
    expect(after.eating).toBe(false);
    expect(after.until).toEqual(at('18:30'));
  });
});

describe('formatDuration', () => {
  it('shows hours and minutes', () => {
    expect(formatDuration(42 * 60_000)).toBe('42 min');
    expect(formatDuration((3 * 60 + 5) * 60_000)).toBe('3 h 05 min');
    expect(formatDuration(-1)).toBe('0 min');
  });
});
