import type { Food } from './diary';

// Amounts the way people think of them: a tablespoon of hummus, a slice of
// bread, a glass of milk, a handful of nuts. Nobody weighs their hummus, so a
// food offers its own serving and the household units that suit it, and grams
// stay one tap away.

/** A unit of a food, such as a tablespoon of 15 g. */
export interface Unit {
  /** the unit's name for one and for more, such as "slice" and "slices" */
  one: string;
  many: string;
  grams: number;
}

const unit = (one: string, grams: number, many = one + 's'): Unit => ({ one, many, grams });
const tbsp = (grams = 15) => unit('tbsp', grams, 'tbsp');
const tsp = (grams = 5) => unit('tsp', grams, 'tsp');
const piece = (grams: number) => unit('piece', grams);

// Typical weights, rounded; the first rule a food's name matches wins, so the
// specific ones (peanut butter) come before the general ones (butter). Names
// are matched in English, Dutch and French, as the food tables have them.
const rules: [RegExp, Unit[]][] = [
  [
    /hummus|houmous|peanut butter|pindakaas|beurre de cacahu|tahini|tahin|nutella|choco(late)? spread|chocopasta|guacamole|tzatziki|pesto|cream cheese|smeerkaas|fromage frais|cottage/,
    [tbsp(), tsp()],
  ],
  [
    /mayo|ketchup|mustard|mosterd|moutarde|sauce|saus|dressing|vinaigrette|soy|soja|sambal|sriracha|salsa/,
    [tbsp(), tsp()],
  ],
  [/jam|confiture|honey|honing|miel|syrup|siroop|sirop|stroop/, [tsp(7), tbsp(21)]],
  [/\boil\b|\bolie\b|huile/, [tbsp(14), tsp(5)]],
  [/butter|boter|beurre|margarine|ghee/, [tsp(5), tbsp(14)]],
  [/sugar|suiker|sucre|cocoa|cacao|flour|bloem|farine/, [tsp(4), tbsp(12)]],
  [/espresso|ristretto/, [unit('shot', 30)]],
  [
    /coffee|koffie|café|cafe|cappuccino|latte|\btea\b|thee|\bthé/,
    [unit('cup', 200), unit('mug', 300)],
  ],
  [/milk|melk|lait/, [unit('splash', 30, 'splashes'), unit('glass', 250, 'glasses'), tbsp()]],
  [/wine|wijn|\bvin\b/, [unit('glass', 150, 'glasses')]],
  [/beer|bier|bière|biere|pils/, [unit('glass', 250, 'glasses'), unit('bottle', 330)]],
  [
    /juice|\bsap\b|\bjus\b|soda|\bcola\b|lemonade|limonade|smoothie|kefir|drink|drank|boisson|\bwater\b/,
    [unit('glass', 250, 'glasses'), unit('can', 330)],
  ],
  [/soup|soep|soupe|ramen|pho\b|broth|bouillon/, [unit('bowl', 300), unit('cup', 240)]],
  [
    /yogh?urt|yaourt|skyr|kwark|quark|pudding|custard|vla/,
    [unit('pot', 125), tbsp(), unit('bowl', 200)],
  ],
  [/pancake|pannenkoek|crêpe|crepe|donut|doughnut|croissant/, [piece(60)]],
  [
    /\boats?\b|haver|avoine|muesli|granola|cereal|cornflakes|porridge|pap\b/,
    [unit('bowl', 50), tbsp(10)],
  ],
  [/rice|rijst|\briz\b|quinoa|couscous|bulgur/, [unit('portion', 180), unit('cup', 160), tbsp(15)]],
  [
    /pasta|spaghetti|penne|macaroni|noodle|nouille|noedel|tagliatelle|fusilli/,
    [unit('plate', 250), unit('cup', 140)],
  ],
  [
    /\bnuts?\b|walnut|hazelnut|noot|noten|noix|almond|amandel|amande|cashew|pistach|peanut|pinda|seed|zaad|graine|raisin|rozijn/,
    [unit('handful', 30), tbsp(10)],
  ],
  [/crisps|chips|popcorn|pretzel/, [unit('handful', 20), unit('bag', 45)]],
  [
    /berr|bes|bessen|baie|grape|druif|druiven|raisin|cherry|kers|cerise|blueberr|strawberr|aardbei|fraise/,
    [unit('handful', 70), unit('cup', 150)],
  ],
  [
    /bread|brood|pain|toast|bagel|baguette|stokbrood|sandwich|boterham|cracker|beschuit|rusk/,
    [unit('slice', 35)],
  ],
  [/pizza/, [unit('slice', 100)]],
  [/cake|taart|tarte|gâteau|gateau|\bpie\b|brownie/, [unit('slice', 80)]],
  [/cookie|biscuit|koek|speculoos|wafel|waffle|gaufre/, [piece(15)]],
  [/chocolate|chocolade|chocolat/, [unit('square', 5), unit('bar', 100)]],
  [/cheese|kaas|fromage|parmesan|mozzarella|feta/, [unit('slice', 20), tbsp(7)]],
  [/\bham\b|salami|bacon|spek|jambon|chorizo|turkey breast|kalkoen/, [unit('slice', 15)]],
  [/\beggs?\b|\bei\b|eieren|œuf|oeuf/, [unit('egg', 50)]],
  [/banana|banaan|banane/, [piece(120)]],
  [/apple|appel|pomme/, [piece(180)]],
  [/\bpears?\b|\bpeer\b|poire/, [piece(170)]],
  [/orange|sinaas/, [piece(150)]],
  [/mandarin|clementine|tangerine/, [piece(70)]],
  [/kiwi/, [piece(75)]],
  [/peach|perzik|pêche|nectarine/, [piece(150)]],
  [/plum|pruim|prune|apricot|abrikoos|abricot/, [piece(50)]],
  [/avocado|avocat/, [unit('half', 75, 'halves'), piece(150)]],
  [/tomato|tomaat|tomate/, [piece(120), unit('cherry tomato', 15, 'cherry tomatoes')]],
  [/carrot|wortel|carotte/, [piece(60)]],
  [/potato|aardappel|pomme de terre/, [piece(150)]],
  [/fries|frites|friet|patat/, [unit('portion', 150), unit('handful', 50)]],
  [/onion|ui\b|uien|oignon/, [piece(110)]],
  [/cucumber|komkommer|concombre/, [unit('slice', 7)]],
  [
    /salad|salade|\bsla\b|spinach|spinazie|épinard|lettuce/,
    [unit('bowl', 80), unit('handful', 30)],
  ],
];

/** The household units that suit a food, from its name. */
export function householdUnits(name: string): Unit[] {
  const n = name.toLowerCase();
  return rules.find(([re]) => re.test(n))?.[1] ?? [];
}

/** A food's serving as a unit: "1 slice" becomes slice and slices. */
export function servingUnit(food: Pick<Food, 'serving_g' | 'serving_label'>): Unit | null {
  if (!(food.serving_g > 0)) return null;
  const label = food.serving_label.trim();
  const simple = label.match(/^1 ([a-z]+)$/i)?.[1];
  if (simple) return unit(simple.toLowerCase(), food.serving_g, plural(simple.toLowerCase()));
  // "2 slices (60 g)" or no label: counted as servings of that
  const what = label || `${food.serving_g} g`;
  return { one: `serving (${what})`, many: `servings (${what})`, grams: food.serving_g };
}

function plural(word: string): string {
  if (/(s|sh|ch|x)$/.test(word)) return word + 'es';
  if (/[^aeiou]y$/.test(word)) return word.slice(0, -1) + 'ies';
  return word + 's';
}

/** The units to offer for a food: its own serving first, then household units of other sizes. */
export function unitsOf(food: Pick<Food, 'name' | 'serving_g' | 'serving_label'>): Unit[] {
  const serving = servingUnit(food);
  const more = householdUnits(food.name).filter(
    (u) => !serving || (u.one !== serving.one && u.grams !== serving.grams),
  );
  return serving ? [serving, ...more] : more;
}

/** "1 tbsp", "2.5 slices". */
export function countText(u: Unit, n: number): string {
  return `${n} ${n === 1 ? u.one : u.many}`;
}

/** Reads an amount such as "2 tbsp" back into one of the units, or null. */
export function readAmount(amount: string, units: Unit[]): { unit: Unit; count: number } | null {
  const m = amount.trim().match(/^(\d+(?:\.\d+)?) (.+)$/);
  if (!m) return null;
  const found = units.find((u) => u.one === m[2] || u.many === m[2]);
  return found ? { unit: found, count: parseFloat(m[1]) } : null;
}

/** A count's grams, to a tenth of a gram. */
export function gramsOf(u: Unit, n: number): number {
  return Math.round(u.grams * n * 10) / 10;
}

/**
 * Where the amount starts for a food: what was logged last time (in units when it
 * comes to a whole or half number of them), else one of its
 * serving or first household unit, else 100 g.
 */
export function startAmount(
  food: Pick<Food, 'name' | 'serving_g' | 'serving_label'>,
  last?: { grams: number; amount: string },
): { grams: number; amount: string } {
  if (last?.grams)
    return { grams: last.grams, amount: last.amount || guessAmount(food, last.grams) };
  const u = unitsOf(food)[0];
  return u ? { grams: u.grams, amount: countText(u, 1) } : { grams: 100, amount: '' };
}

/** An amount in grams as a count of the food's first unit when it is a whole or half number of them, else "". */
export function guessAmount(
  food: Pick<Food, 'name' | 'serving_g' | 'serving_label'>,
  grams: number,
): string {
  const u = unitsOf(food)[0];
  if (!u || !grams) return '';
  const n = grams / u.grams;
  return n >= 0.5 && Math.abs(n * 2 - Math.round(n * 2)) < 0.01
    ? countText(u, Math.round(n * 2) / 2)
    : '';
}
