import { describe, expect, it, vi } from 'vitest';
import { importExport, PART_SIZE, summary, type ImportResult } from './loseit';
import { pb } from './pb';

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

describe('importExport', () => {
  it('sends the file in parts, then imports it', async () => {
    const file = new Blob([new Uint8Array(PART_SIZE * 2 + 10)]);
    const calls: { url: string; offset: string | null; size: number; plan: string | null }[] = [];
    let lost = true;
    const send = vi.spyOn(pb, 'send').mockImplementation(async (url, options) => {
      const body = options?.body as FormData;
      const part = body.get('part') as Blob | null;
      calls.push({
        url,
        offset: body.get('offset') as string | null,
        size: part?.size ?? 0,
        plan: body.get('plan') as string | null,
      });
      if (!part) return { entries: { added: 1, existing: 0 } };
      const offset = Number(body.get('offset'));
      // the answer to the second part gets lost, and the server already has it
      if (offset === PART_SIZE && lost) {
        lost = false;
        throw { status: 0 };
      }
      return { size: offset + part.size };
    });
    const progress: number[] = [];
    const r = await importExport(file, true, (p) => progress.push(p));

    expect(r.entries.added).toBe(1);
    expect(calls.map((c) => [c.offset, c.size])).toEqual([
      ['0', PART_SIZE],
      [String(PART_SIZE), PART_SIZE],
      [String(PART_SIZE), PART_SIZE],
      [String(PART_SIZE * 2), 10],
      [null, 0],
    ]);
    expect(calls.at(-1)).toMatchObject({ url: '/api/app/import/loseit', plan: 'true' });
    expect(progress.at(-1)).toBe(1);
    send.mockRestore();
  });
});
