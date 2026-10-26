import { describe, expect, it } from 'vitest';
import { summary, type ImportResult } from './loseit';

const none = { added: 0, existing: 0 };
const result = (r: Partial<ImportResult>): ImportResult => ({
  entries: none,
  weighins: none,
  exercises: none,
  foods: none,
  photos: none,
  plan: false,
  unweighed: 0,
  unused: [],
  ...r,
});

describe('summary', () => {
  it('says what came in', () => {
    expect(
      summary(
        result({
          entries: { added: 1319, existing: 0 },
          weighins: { added: 24, existing: 1 },
          photos: { added: 1, existing: 0 },
          plan: true,
          unused: ['achievements (20)', 'achievement-actions (3)', 'garmin-calories (66)'],
        }),
      ),
    ).toEqual([
      'Added 1,319 foods in your diary, 24 weigh-ins and 1 meal photo.',
      'Skipped 1 already in Troye.',
      'Took your plan and fasting schedule from Lose It!',
      "Not brought over: badges and Garmin's daily totals (its extra calories came in as exercise).",
    ]);
  });

  it('says when there was nothing new', () => {
    expect(summary(result({ entries: { added: 0, existing: 3 }, unweighed: 2 }))).toEqual([
      'Nothing new to add.',
      'Skipped 3 already in Troye.',
      "2 foods of your own were left out, because Lose It! doesn't say what a serving weighs.",
    ]);
  });
});
