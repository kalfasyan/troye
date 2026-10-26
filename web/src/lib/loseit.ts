// Importing a Lose It! export (POST /api/app/import/loseit, backend/app/loseit.go).

export interface Counted {
  added: number;
  existing: number;
}

/** What an import brought in. */
export interface ImportResult {
  entries: Counted;
  weighins: Counted;
  exercises: Counted;
  foods: Counted;
  photos: Counted;
  plan: boolean;
  plan_error?: string;
  unweighed: number;
  unused: string[];
}

const kinds: { key: keyof ImportResult; one: string; many: string }[] = [
  { key: 'entries', one: 'food in your diary', many: 'foods in your diary' },
  { key: 'weighins', one: 'weigh-in', many: 'weigh-ins' },
  { key: 'exercises', one: 'exercise', many: 'exercises' },
  { key: 'foods', one: 'food of your own', many: 'foods of your own' },
  { key: 'photos', one: 'meal photo', many: 'meal photos' },
];

// Lose It! files Troye has no place for, by what they hold
const unusedNames: Record<string, string> = {
  achievements: 'badges',
  'achievement-actions': 'badges',
  'course-progress': 'lessons',
  'garmin-calories': "Garmin's daily totals (its extra calories came in as exercise)",
  notes: 'notes',
  'weight-loss-medication-logs': 'medication',
  'progress-photos': 'progress photos',
  'lab-report-entries': 'lab reports',
  'fasting-logs': 'past fasts',
  focuses: 'focus goals',
  'custom-exercises': 'your own exercises',
};

const count = (n: number, one: string, many: string) =>
  `${n.toLocaleString('en')} ${n === 1 ? one : many}`;

/** The import's outcome as a few sentences, what came in first. */
export function summary(r: ImportResult): string[] {
  const added = kinds
    .filter((k) => (r[k.key] as Counted).added)
    .map((k) => count((r[k.key] as Counted).added, k.one, k.many));
  const lines = [added.length ? `Added ${list(added)}.` : 'Nothing new to add.'];
  const existing = kinds.reduce((n, k) => n + (r[k.key] as Counted).existing, 0);
  if (existing) lines.push(`Skipped ${existing.toLocaleString('en')} already in Troye.`);
  if (r.plan) lines.push('Took your plan and fasting schedule from Lose It!');
  if (r.plan_error) lines.push(r.plan_error);
  if (r.unweighed) {
    lines.push(
      `${count(r.unweighed, 'food of your own was', 'foods of your own were')} left out, because Lose It! doesn't say what a serving weighs.`,
    );
  }
  const unused = [...new Set(r.unused.map((u) => unusedNames[u.replace(/ \(\d+\)$/, '')] ?? u))];
  if (unused.length) lines.push(`Not brought over: ${list(unused)}.`);
  return lines;
}

function list(items: string[]): string {
  if (items.length <= 1) return items.join('');
  return `${items.slice(0, -1).join(', ')} and ${items.at(-1)}`;
}
