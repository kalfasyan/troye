// How good the photo analyser's drafts are, judged by what people logged in
// the end. workers/evaluate.py scores other models the same way.

export interface Draft {
  food: string;
  grams: number;
  kcal: number;
}

export interface Logged {
  food: string;
  grams: number;
  kcal: number;
}

export interface PhotoScore {
  /** the draft's calories against what was logged, as a fraction: 0.1 is 10% over */
  kcal: number | null;
  /** foods logged that the draft had, and foods logged */
  found: number;
  logged: number;
  /** draft items that weren't kept */
  extra: number;
  drafted: number;
  /** for each food the draft had right, how far off its grams were, as a fraction */
  grams: number[];
}

export function scorePhoto(draft: Draft[], logged: Logged[]): PhotoScore {
  const draftKcal = draft.reduce((s, i) => s + i.kcal, 0);
  const loggedKcal = logged.reduce((s, i) => s + i.kcal, 0);
  const keptFoods = new Set(logged.map((i) => i.food).filter(Boolean));
  const score: PhotoScore = {
    kcal: loggedKcal > 0 ? (draftKcal - loggedKcal) / loggedKcal : null,
    found: 0,
    logged: logged.filter((i) => i.food).length,
    extra: draft.filter((i) => !i.food || !keptFoods.has(i.food)).length,
    drafted: draft.length,
    grams: [],
  };
  for (const l of logged) {
    if (!l.food) continue;
    const d = draft.find((i) => i.food === l.food);
    if (!d) continue;
    score.found++;
    if (l.grams > 0) score.grams.push(Math.abs(d.grams - l.grams) / l.grams);
  }
  return score;
}

export interface ModelScore {
  model: string;
  photos: number;
  /** the typical calorie miss either way, as a fraction */
  kcalOff: number | null;
  /** over (+) or under (−) on average, as a fraction */
  kcalBias: number | null;
  /** the share of logged foods the drafts had */
  found: number | null;
  /** the share of draft items that were dropped */
  extra: number | null;
  /** the typical portion miss on foods it got right, as a fraction */
  gramsOff: number | null;
}

const mean = (xs: number[]) => (xs.length ? xs.reduce((s, x) => s + x, 0) / xs.length : null);

const median = (xs: number[]) => {
  if (!xs.length) return null;
  const s = [...xs].sort((a, b) => a - b);
  const m = Math.floor(s.length / 2);
  return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2;
};

/** One line per model, the most photos first. */
export function scoreModels(
  photos: { model: string; items: Draft[] | null; confirmed: Logged[] | null }[],
): ModelScore[] {
  const by = new Map<string, PhotoScore[]>();
  for (const p of photos) {
    if (!p.items || !p.confirmed) continue;
    const model = p.model || 'unknown';
    by.set(model, [...(by.get(model) ?? []), scorePhoto(p.items, p.confirmed)]);
  }
  return [...by]
    .map(([model, scores]) => {
      const kcal = scores.flatMap((s) => (s.kcal === null ? [] : [s.kcal]));
      const logged = scores.reduce((n, s) => n + s.logged, 0);
      const drafted = scores.reduce((n, s) => n + s.drafted, 0);
      return {
        model,
        photos: scores.length,
        kcalOff: median(kcal.map(Math.abs)),
        kcalBias: mean(kcal),
        found: logged ? scores.reduce((n, s) => n + s.found, 0) / logged : null,
        extra: drafted ? scores.reduce((n, s) => n + s.extra, 0) / drafted : null,
        gramsOff: median(scores.flatMap((s) => s.grams)),
      };
    })
    .sort((a, b) => b.photos - a.photos);
}

/** A fraction as a whole percentage, such as "12%". */
export const pct = (x: number | null) => (x === null ? '–' : `${Math.round(Math.abs(x) * 100)}%`);
