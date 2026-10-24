import { describe, expect, it } from 'vitest';
import { draftKcal, gramStep, itemTotals, sha256, statusText } from './photos';

describe('photo drafts', () => {
  it('works out an item at some grams', () => {
    expect(itemTotals({ kcal: 130, protein: 2.7, carbs: 28, fat: 0.3 }, 150)).toEqual({
      kcal: 195,
      protein: 4.1,
      carbs: 42,
      fat: 0.5,
    });
  });

  it('rounds the draft to about 10 kcal', () => {
    expect(draftKcal([{ kcal: 234 }, { kcal: 198 }])).toBe(430);
    expect(draftKcal(null)).toBe(0);
  });

  it('says where a photo is', () => {
    expect(statusText({ status: 'queued', items: null }, false)).toBe('Waiting for your PC');
    expect(statusText({ status: 'queued', items: null }, true)).toBe('In the queue');
    expect(statusText({ status: 'ready', items: [{ kcal: 640 } as never] }, true)).toBe(
      'Ready to check · about 640 kcal',
    );
  });

  it('steps grams by portion size', () => {
    expect([30, 120, 300].map(gramStep)).toEqual([5, 10, 25]);
  });

  it('hashes files', async () => {
    expect(await sha256(new Blob(['abc']))).toBe(
      'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad',
    );
  });
});
