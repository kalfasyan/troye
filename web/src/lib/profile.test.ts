import { describe, expect, it } from 'vitest';
import {
  blankAnswers,
  cmToFtIn,
  eatingEnd,
  formatWeight,
  ftInToCm,
  kgToLb,
  lbToKg,
  planRequest,
} from './profile';

describe('units', () => {
  it('converts weights both ways', () => {
    expect(kgToLb(100)).toBeCloseTo(220.46, 2);
    expect(lbToKg(kgToLb(72.5))).toBeCloseTo(72.5, 6);
  });

  it('converts heights both ways', () => {
    expect(cmToFtIn(180)).toEqual([5, 11]);
    expect(ftInToCm(5, 11)).toBeCloseTo(180.34, 2);
  });

  it('formats weights in the chosen units', () => {
    expect(formatWeight(80.04, 'metric')).toBe('80 kg');
    expect(formatWeight(80, 'imperial')).toBe('176 lb');
  });
});

describe('eatingEnd', () => {
  it('closes the window after 24 hours minus the fast', () => {
    expect(eatingEnd('12:00', '16_8')).toBe('20:00');
    expect(eatingEnd('18:30', '16_8')).toBe('02:30');
    expect(eatingEnd('09:15', 'none')).toBe('09:15');
  });
});

describe('planRequest', () => {
  it('keeps the current weight and no pace when maintaining', () => {
    const a = { ...blankAnswers(), goal: 'maintain' as const, weight_kg: 70, goal_weight_kg: 60 };
    expect(planRequest(a)).toMatchObject({ goal_weight_kg: 70, pace_kg: 0 });
  });
});
