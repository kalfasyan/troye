import { describe, expect, it } from 'vitest';
import type { Food } from './diary';
import {
  countText,
  guessAmount,
  householdUnits,
  readAmount,
  servingUnit,
  startAmount,
  unitsOf,
} from './portions';

const food = (name: string, extra: Partial<Food> = {}) =>
  ({ name, serving_g: 0, serving_label: '', ...extra }) as Food;

describe('household units', () => {
  it('suits the food', () => {
    expect(householdUnits('Hummus, plain').map((u) => u.one)).toEqual(['tbsp', 'tsp']);
    expect(householdUnits('Pindakaas')[0].one).toBe('tbsp');
    expect(householdUnits('Milk, semi-skimmed')[0].one).toBe('splash');
    expect(householdUnits('Wholemeal bread')[0]).toEqual({
      one: 'slice',
      many: 'slices',
      grams: 35,
    });
    expect(householdUnits('Almonds')[0].one).toBe('handful');
    expect(householdUnits('Banana')[0].one).toBe('piece');
    expect(householdUnits('Zzz')).toEqual([]);
  });

  it("doesn't trip over words inside words", () => {
    expect(householdUnits('Egg, boiled')[0].one).toBe('egg');
    expect(householdUnits('Peanut butter')[0].one).toBe('tbsp');
    expect(householdUnits('Oat milk')[0].one).toBe('splash');
    expect(householdUnits('Chocolate, dark')[0].one).toBe('square');
    expect(householdUnits('Watermelon')).toEqual([]);
  });
});

describe('units of a food', () => {
  it('puts its own serving first', () => {
    const bread = food('Bread', { serving_g: 30, serving_label: '1 slice' });
    expect(unitsOf(bread)).toEqual([{ one: 'slice', many: 'slices', grams: 30 }]);
    const hummus = food('Hummus', { serving_g: 50, serving_label: '1 pot' });
    expect(unitsOf(hummus).map((u) => u.one)).toEqual(['pot', 'tbsp', 'tsp']);
  });

  it('counts a serving without a simple name as servings', () => {
    expect(servingUnit(food('x', { serving_g: 60, serving_label: '2 slices' }))?.many).toBe(
      'servings (2 slices)',
    );
    expect(servingUnit(food('x', { serving_g: 0, serving_label: '1 slice' }))).toBeNull();
    expect(servingUnit(food('x', { serving_g: 25, serving_label: '1 Biscuit' }))?.many).toBe(
      'biscuits',
    );
  });

  it('writes and reads amounts', () => {
    const units = unitsOf(food('Hummus'));
    expect(countText(units[0], 1)).toBe('1 tbsp');
    expect(readAmount('2.5 tbsp', units)).toEqual({ unit: units[0], count: 2.5 });
    expect(readAmount('2 Tablespoons', units)).toBeNull();
    expect(readAmount('', units)).toBeNull();
  });

  it('starts from the last amount, else one unit, else 100 g', () => {
    expect(startAmount(food('Hummus'), { grams: 45, amount: '3 tbsp' })).toEqual({
      grams: 45,
      amount: '3 tbsp',
    });
    expect(startAmount(food('Hummus'), { grams: 30, amount: '' }).amount).toBe('2 tbsp');
    expect(startAmount(food('Hummus'))).toEqual({ grams: 15, amount: '1 tbsp' });
    expect(startAmount(food('Zzz'))).toEqual({ grams: 100, amount: '' });
  });

  it('guesses a count for grams logged before units', () => {
    const bread = food('Bread', { serving_g: 35, serving_label: '1 slice' });
    expect(guessAmount(bread, 70)).toBe('2 slices');
    expect(guessAmount(bread, 52.5)).toBe('1.5 slices');
    expect(guessAmount(bread, 50)).toBe('');
  });
});
