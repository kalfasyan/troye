import { describe, expect, it } from 'vitest';
import {
  addDays,
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
