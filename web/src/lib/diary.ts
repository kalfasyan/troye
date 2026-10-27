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
  /** as it was logged, such as "2 Tablespoons", for imported entries */
  amount?: string;
  fiber?: number;
  sugar?: number;
  sat_fat?: number;
  salt?: number;
  /** milligrams */
  cholesterol?: number;
  /** the entry this one was added to, like the coffee for its milk */
  parent?: string;
  expand?: { food?: Food };
}

/** How much of something was eaten: as logged, or in grams. */
export function entryAmount(e: Pick<Entry, 'amount' | 'grams'>): string {
  return e.amount || (e.grams ? `${e.grams} g` : '');
}

/** The nutrients beyond the macros that an entry has, to show with it. */
export function moreNutrients(
  e: Pick<Entry, 'fiber' | 'sugar' | 'sat_fat' | 'salt' | 'cholesterol'>,
): string[] {
  const list: [string, number | undefined, string][] = [
    ['fiber', e.fiber, 'g'],
    ['sugar', e.sugar, 'g'],
    ['saturated fat', e.sat_fat, 'g'],
    ['salt', e.salt, 'g'],
    ['cholesterol', e.cholesterol, 'mg'],
  ];
  return list.filter(([, v]) => v).map(([label, v, unit]) => `${v} ${unit} ${label}`);
}

/** How many servings an amount of a food is, to one decimal, or 0 for a food without servings. */
export function servingsOf(food: Pick<Food, 'serving_g'>, grams: number): number {
  return food.serving_g > 0 ? Math.round((grams / food.serving_g) * 10) / 10 : 0;
}

/** One step up or down from a number of servings: halves below 2, whole ones above. */
export function stepServings(n: number, up: boolean): number {
  const step = (up ? n < 2 : n <= 2) ? 0.5 : 1;
  const next = up ? Math.floor(n / step) * step + step : Math.ceil(n / step) * step - step;
  return Math.max(0.5, Math.round(next * 10) / 10);
}

/**
 * An entry without a food (a quick add or an import) times a factor: its values,
 * and its amount when it starts with a number ("2 Tablespoons" becomes "3 Tablespoons").
 */
export function scaleEntry(
  e: Pick<Entry, 'kcal' | 'protein' | 'carbs' | 'fat' | 'amount'>,
  factor: number,
): Pick<Entry, 'kcal' | 'protein' | 'carbs' | 'fat' | 'amount'> {
  const r = (n: number) => Math.round((n || 0) * factor * 10) / 10;
  const amount = (e.amount ?? '').replace(/^(\d+(?:[.,]\d+)?)/, (n) => {
    const v = Math.round(parseFloat(n.replace(',', '.')) * factor * 100) / 100;
    return String(v);
  });
  return {
    kcal: Math.round((e.kcal || 0) * factor),
    protein: r(e.protein),
    carbs: r(e.carbs),
    fat: r(e.fat),
    amount,
  };
}

/** Things people often add to a food, to search for: milk for a coffee, an egg for a soup. */
export function addOnIdeas(name: string): string[] {
  const n = name.toLowerCase();
  const ideas: [RegExp, string[]][] = [
    [/coffee|espresso|cappuccino|latte|koffie|café|cafe/, ['milk', 'oat milk', 'sugar', 'cream']],
    [/\btea\b|thee|thé/, ['milk', 'sugar', 'honey', 'lemon']],
    [
      /noodle|ramen|pho|soup|soep|soupe|udon|nouille/,
      ['egg', 'tofu', 'chicken', 'spring onion', 'chili oil', 'bean sprouts'],
    ],
    [/salad|salade|sla\b/, ['dressing', 'olive oil', 'cheese', 'croutons', 'avocado']],
    [
      /yogh?urt|skyr|kwark|quark|porridge|oat|muesli|granola|cereal/,
      ['honey', 'banana', 'berries', 'nuts', 'milk'],
    ],
    [/pasta|spaghetti|penne|rice|rijst|riz/, ['parmesan', 'olive oil', 'sauce', 'chicken']],
    [
      /bread|brood|pain|toast|sandwich|bagel|cracker/,
      ['butter', 'cheese', 'ham', 'jam', 'peanut butter'],
    ],
    [
      /pancake|pannenkoek|crêpe|crepe|waffle|wafel|gaufre/,
      ['syrup', 'sugar', 'nutella', 'whipped cream'],
    ],
    [/potato|aardappel|frites|fries|patat/, ['mayonnaise', 'ketchup', 'butter']],
    [/pizza|burger|wrap|taco/, ['cheese', 'sauce', 'fries']],
  ];
  return ideas.find(([re]) => re.test(n))?.[1] ?? [];
}

/** The colors of the macros, the same on the ring and the bars under it. */
export const macroColors = {
  carbs: { stroke: 'stroke-orange-500', bg: 'bg-orange-500' },
  protein: { stroke: 'stroke-teal-500', bg: 'bg-teal-500' },
  fat: { stroke: 'stroke-amber-400', bg: 'bg-amber-400' },
  other: { stroke: 'stroke-zinc-400', bg: 'bg-zinc-400' },
} as const;

/** The day's calories split by where they came from, for the ring: carbs, protein, fat, and the rest. */
export function calorieSources(t: Totals): {
  carbs: number;
  protein: number;
  fat: number;
  other: number;
} {
  const carbs = t.carbs * 4;
  const protein = t.protein * 4;
  const fat = t.fat * 9;
  const fromMacros = carbs + protein + fat;
  const kcal = Math.max(t.kcal, 0);
  if (!kcal) return { carbs: 0, protein: 0, fat: 0, other: 0 };
  // quick adds without macros (or alcohol) leave calories the macros don't explain;
  // a few percent either way is rounding, so the macros share all of it
  const other = kcal - fromMacros > kcal * 0.05 ? kcal - fromMacros : 0;
  const scale = fromMacros ? (kcal - other) / fromMacros : 0;
  return { carbs: carbs * scale, protein: protein * scale, fat: fat * scale, other };
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
