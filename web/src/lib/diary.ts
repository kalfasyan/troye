import type { RecordModel } from 'pocketbase';

// The food diary's data, and the sums the diary screen shows.

export type Meal = 'breakfast' | 'lunch' | 'dinner' | 'snacks';

export const meals: { id: Meal; label: string }[] = [
  { id: 'breakfast', label: 'Breakfast' },
  { id: 'lunch', label: 'Lunch' },
  { id: 'dinner', label: 'Dinner' },
  { id: 'snacks', label: 'Snacks' },
];

/** A food, with nutrition values per 100 g. */
export interface Food extends RecordModel {
  name: string;
  name_nl?: string;
  name_fr?: string;
  brand: string;
  source: 'custom' | 'off' | 'ciqual' | 'nevo' | 'usda';
  barcode: string;
  kcal: number;
  protein: number;
  carbs: number;
  fat: number;
  serving_g: number;
  serving_label: string;
  owner: string;
}

/** Something eaten, with its totals (the server works them out from the food). */
export interface Entry extends RecordModel {
  user: string;
  date: string;
  meal: Meal;
  food: string;
  name: string;
  grams: number;
  kcal: number;
  protein: number;
  carbs: number;
  fat: number;
  expand?: { food?: Food };
}

export interface Totals {
  kcal: number;
  protein: number;
  carbs: number;
  fat: number;
}

export function sum(entries: Pick<Entry, keyof Totals>[]): Totals {
  const t = { kcal: 0, protein: 0, carbs: 0, fat: 0 };
  for (const e of entries) {
    t.kcal += e.kcal;
    t.protein += e.protein;
    t.carbs += e.carbs;
    t.fat += e.fat;
  }
  return t;
}

export type FoodLanguage = 'en' | 'nl' | 'fr';

export const foodLanguages: { id: FoodLanguage; label: string }[] = [
  { id: 'en', label: 'English' },
  { id: 'nl', label: 'Nederlands' },
  { id: 'fr', label: 'Français' },
];

/** A food's name in the person's language, or its English name; the server does the same for entries. */
export function foodName(
  food: Pick<Food, 'name' | 'name_nl' | 'name_fr'>,
  lang: FoodLanguage | '' | undefined,
): string {
  return (lang === 'nl' ? food.name_nl : lang === 'fr' ? food.name_fr : '') || food.name;
}

/** What an amount of a food comes to; the server does the same for saved entries. */
export function amountOf(food: Pick<Food, keyof Totals>, grams: number): Totals {
  const f = grams / 100;
  const r = (n: number) => Math.round(n * f * 10) / 10;
  return {
    kcal: Math.round(food.kcal * f),
    protein: r(food.protein),
    carbs: r(food.carbs),
    fat: r(food.fat),
  };
}

/** Quick amounts to offer for a food: its serving, then round numbers of grams. */
export function portions(
  food: Pick<Food, 'serving_g' | 'serving_label'>,
): { label: string; grams: number }[] {
  const list = [50, 100, 150, 200].map((g) => ({ label: `${g} g`, grams: g }));
  if (food.serving_g > 0) {
    const label = food.serving_label || `1 serving`;
    return [
      { label: `${label}`, grams: food.serving_g },
      ...list.filter((p) => p.grams !== food.serving_g),
    ];
  }
  return list;
}

/** A day as YYYY-MM-DD in local time. */
export function isoDay(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

export function addDays(day: string, n: number): string {
  const d = new Date(day + 'T12:00:00');
  d.setDate(d.getDate() + n);
  return isoDay(d);
}

/** "Today", "Yesterday", "Tomorrow" or a short date. */
export function dayLabel(day: string, today: string): string {
  if (day === today) return 'Today';
  if (day === addDays(today, -1)) return 'Yesterday';
  if (day === addDays(today, 1)) return 'Tomorrow';
  return new Date(day + 'T12:00:00').toLocaleDateString(undefined, {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
  });
}

/** Recent foods from the newest entries, each food once. */
export function recentFoods(entries: Entry[], limit = 20): Food[] {
  const seen = new Set<string>();
  const foods: Food[] = [];
  for (const e of entries) {
    const f = e.expand?.food;
    if (!f || seen.has(f.id)) continue;
    seen.add(f.id);
    foods.push(f);
    if (foods.length === limit) break;
  }
  return foods;
}
