import { describe, expect, it } from 'vitest';
import {
  ago,
  analyserText,
  draftKcal,
  gramStep,
  itemTotals,
  sha256,
  statusText,
  type AnalyserStatus,
} from './photos';

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

  it('says how long ago', () => {
    const now = new Date('2026-10-26T12:00:00Z');
    expect(ago('2026-10-26T11:59:30Z', now)).toBe('just now');
    expect(ago('2026-10-26T11:45:00Z', now)).toBe('15 min ago');
    expect(ago('2026-10-26T09:00:00Z', now)).toBe('3 hours ago');
    expect(ago('2026-10-25T09:00:00Z', now)).toBe('yesterday');
    expect(ago('2026-10-20T09:00:00Z', now)).toBe('6 days ago');
  });

  it('says plainly where the analyser is', () => {
    const now = new Date('2026-10-26T12:00:00Z');
    const off: AnalyserStatus = {
      online: false,
      set_up: true,
      waiting: 1,
      can_wake: false,
      seen: '',
    };
    expect(analyserText({ ...off, online: true }, false).title).toBe('Your PC is on');
    expect(analyserText({ ...off, set_up: false }, false).title).toBe(
      "Photo analysis isn't set up yet",
    );
    expect(analyserText(off, true).title).toBe('Waking your PC…');
    expect(analyserText({ ...off, seen: '2026-10-26T09:00:00Z' }, false, now)).toEqual({
      title: 'Your PC is off',
      detail: "Last seen 3 hours ago. Photos wait here until it's back on.",
    });
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
