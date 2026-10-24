import { describe, expect, it } from 'vitest';
import { pct, scoreModels, scorePhoto } from './accuracy';

const rice = { food: 'rice', grams: 180, kcal: 234 };
const chicken = { food: 'chicken', grams: 130, kcal: 215 };

describe('scoring a photo', () => {
  it('compares the draft with what was logged', () => {
    const s = scorePhoto(
      [rice, chicken, { food: '', grams: 40, kcal: 0 }],
      [{ ...rice, grams: 150, kcal: 195 }, chicken, { food: '', grams: 0, kcal: 80 }],
    );
    // 449 drafted against 490 logged
    expect(s.kcal).toBeCloseTo(-41 / 490);
    expect([s.found, s.logged, s.extra, s.drafted]).toEqual([2, 2, 1, 3]);
    expect(s.grams).toEqual([0.2, 0]);
  });

  it('counts foods it missed', () => {
    const s = scorePhoto([rice], [rice, chicken]);
    expect([s.found, s.logged, s.extra]).toEqual([1, 2, 0]);
  });
});

describe('scoring models', () => {
  it('sums up each model', () => {
    const [a, b] = scoreModels([
      { model: 'a', items: [rice], confirmed: [rice] },
      { model: 'a', items: [{ ...rice, kcal: 260 }], confirmed: [rice, chicken] },
      { model: 'a', items: null, confirmed: null },
      { model: 'b', items: [chicken], confirmed: [{ ...chicken, grams: 100, kcal: 165 }] },
    ]);
    expect(a.model).toBe('a');
    expect(a.photos).toBe(2);
    expect(a.found).toBeCloseTo(2 / 3);
    expect(a.kcalBias! < 0).toBe(true);
    expect(b.gramsOff).toBeCloseTo(0.3);
    expect(pct(b.kcalOff)).toBe('30%');
    expect(pct(null)).toBe('–');
  });
});
