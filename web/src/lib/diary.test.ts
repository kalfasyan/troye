import { describe, expect, it } from 'vitest';
import {
  addDays,
  addOnIdeas,
  calorieSources,
  scaleEntry,
  servingsOf,
  stepServings,
  amountOf,
  dayLabel,
  entryAmount,
  foodName,
  moreNutrients,
  portions,
  recentFoods,
  sum,
  type Entry,
  type Food,
} from './diary';

const food = (id: string, extra: Partial<Food> = {}) =>
  ({
    id,
    name: id,
    kcal: 130,
    protein: 2.7,
    carbs: 28,
    fat: 0.3,
    serving_g: 0,
    serving_label: '',
    ...extra,
  }) as Food;

describe('amounts', () => {
  it('scales the values per 100 g', () => {
    expect(amountOf(food('rice'), 250)).toEqual({ kcal: 325, protein: 6.8, carbs: 70, fat: 0.8 });
  });

  it('adds entries up', () => {
    const t = sum([
      { kcal: 100, protein: 1, carbs: 2, fat: 3 },
      { kcal: 50, protein: 0.5, carbs: 0, fat: 1 },
    ]);
    expect(t).toEqual({ kcal: 150, protein: 1.5, carbs: 2, fat: 4 });
  });

  it('offers the serving first', () => {
    expect(portions(food('a')).map((p) => p.grams)).toEqual([50, 100, 150, 200]);
    const biscuit = portions(food('b', { serving_g: 6.25, serving_label: '1 biscuit' }));
    expect(biscuit[0]).toEqual({ label: '1 biscuit', grams: 6.25 });
    expect(portions(food('c', { serving_g: 100 })).map((p) => p.grams)).toEqual([
      100, 50, 150, 200,
    ]);
  });
});

describe('days', () => {
  it('moves across months and labels nearby days', () => {
    expect(addDays('2026-10-31', 1)).toBe('2026-11-01');
    expect(addDays('2026-03-01', -1)).toBe('2026-02-28');
    expect(dayLabel('2026-10-24', '2026-10-24')).toBe('Today');
    expect(dayLabel('2026-10-23', '2026-10-24')).toBe('Yesterday');
  });
});

describe('recentFoods', () => {
  it('lists each food once, newest first', () => {
    const entry = (f?: Food) => ({ expand: f ? { food: f } : {} }) as Entry;
    const rice = food('rice');
    const soup = food('soup');
    expect(recentFoods([entry(rice), entry(), entry(soup), entry(rice)]).map((f) => f.id)).toEqual([
      'rice',
      'soup',
    ]);
  });
});

describe('foodName', () => {
  const rice = { name: 'Rice, cooked', name_nl: 'Rijst, gekookt', name_fr: '' };
  it('shows the chosen language, else English', () => {
    expect(foodName(rice, 'nl')).toBe('Rijst, gekookt');
    expect(foodName(rice, 'fr')).toBe('Rice, cooked');
    expect(foodName(rice, '')).toBe('Rice, cooked');
  });
});

describe('imported entries', () => {
  it('show the amount as logged, or the grams', () => {
    expect(entryAmount({ amount: '2 Tablespoons', grams: 0 })).toBe('2 Tablespoons');
    expect(entryAmount({ amount: '', grams: 150 })).toBe('150 g');
    expect(entryAmount({ grams: 0 })).toBe('');
  });

  it('list the nutrients they have', () => {
    expect(moreNutrients({ fiber: 3.1, sugar: 0, salt: 0.65, cholesterol: 40 })).toEqual([
      '3.1 g fiber',
      '0.65 g salt',
      '40 mg cholesterol',
    ]);
  });
});

describe('servings', () => {
  it('counts servings of a food', () => {
    expect(servingsOf({ serving_g: 30 }, 60)).toBe(2);
    expect(servingsOf({ serving_g: 30 }, 45)).toBe(1.5);
    expect(servingsOf({ serving_g: 0 }, 45)).toBe(0);
  });

  it('steps by halves up to 2, then by whole servings', () => {
    expect(stepServings(1, true)).toBe(1.5);
    expect(stepServings(1.5, true)).toBe(2);
    expect(stepServings(2, true)).toBe(3);
    expect(stepServings(3, false)).toBe(2);
    expect(stepServings(2, false)).toBe(1.5);
    expect(stepServings(0.5, false)).toBe(0.5);
    // an odd amount snaps to the next step
    expect(stepServings(1.3, true)).toBe(1.5);
    expect(stepServings(1.3, false)).toBe(1);
  });

  it('scales an entry without a food, and the number in its amount', () => {
    const e = { kcal: 120, protein: 3, carbs: 20.5, fat: 2, amount: '2 Tablespoons' };
    expect(scaleEntry(e, 1.5)).toEqual({
      kcal: 180,
      protein: 4.5,
      carbs: 30.8,
      fat: 3,
      amount: '3 Tablespoons',
    });
    expect(scaleEntry({ ...e, amount: 'a bowl' }, 2).amount).toBe('a bowl');
    expect(scaleEntry({ ...e, amount: '0,5 cup' }, 3).amount).toBe('1.5 cup');
  });
});

describe('add-ons', () => {
  it('suggests what goes with a food', () => {
    expect(addOnIdeas('Coffee, black')).toContain('milk');
    expect(addOnIdeas('Ramen noodle soup')).toContain('egg');
    expect(addOnIdeas('Apple')).toEqual([]);
  });
});

describe('calorie sources', () => {
  it('splits the calories by macro', () => {
    const s = calorieSources({ kcal: 400, carbs: 50, protein: 25, fat: 0 });
    expect(s).toEqual({ carbs: 200, protein: 100, fat: 0, other: 100 });
  });

  it('shares rounding between the macros', () => {
    const s = calorieSources({ kcal: 100, carbs: 10, protein: 10, fat: 2.1 });
    expect(s.other).toBe(0);
    expect(s.carbs + s.protein + s.fat).toBeCloseTo(100);
  });

  it('leaves calories without macros as other', () => {
    expect(calorieSources({ kcal: 300, carbs: 0, protein: 0, fat: 0 }).other).toBe(300);
    expect(calorieSources({ kcal: 0, carbs: 0, protein: 0, fat: 0 }).other).toBe(0);
  });
});
