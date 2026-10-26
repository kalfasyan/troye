import type { RecordModel } from 'pocketbase';
import type { Meal, Totals } from './diary';

// Meal photos and their drafts (backend/app/photos.go). The analyser names
// each item and its grams; the server matches it to a food; the person checks
// the draft and logs it.

export type PhotoStatus = 'queued' | 'analysing' | 'ready' | 'logged' | 'failed';

export interface Per100 {
  kcal: number;
  protein: number;
  carbs: number;
  fat: number;
}

export interface DraftItem {
  /** what the analyser called it */
  name: string;
  grams: number;
  confidence: number;
  /** the matched food, empty when nothing matched */
  food: string;
  food_name: string;
  kcal: number;
  per100: Per100;
  alternatives: { food: string; name: string; per100: Per100 }[];
}

export interface MealPhoto extends RecordModel {
  user: string;
  date: string;
  meal: Meal;
  photo: string;
  hash: string;
  note: string;
  status: PhotoStatus;
  items: DraftItem[] | null;
  /** what was logged in the end */
  confirmed: { name: string; food: string; grams: number; kcal: number; entry?: string }[] | null;
  model: string;
  error: string;
}

export interface AnalyserStatus {
  online: boolean;
  /** whether the analyser has ever asked for work or finished a photo */
  set_up: boolean;
  waiting: number;
  can_wake: boolean;
  seen: string;
}

/** How long ago a time was, in words: "just now", "5 min ago", "3 days ago". */
export function ago(iso: string, now = new Date()): string {
  const min = Math.floor((now.getTime() - new Date(iso).getTime()) / 60_000);
  if (min < 2) return 'just now';
  if (min < 60) return `${min} min ago`;
  const h = Math.floor(min / 60);
  if (h < 24) return h === 1 ? 'an hour ago' : `${h} hours ago`;
  const d = Math.floor(h / 24);
  return d === 1 ? 'yesterday' : `${d} days ago`;
}

/** A plain line about the analyser on the PC, and what it means for photos. */
export function analyserText(
  a: AnalyserStatus,
  woke: boolean,
  now = new Date(),
): { title: string; detail: string } {
  if (a.online) return { title: 'Your PC is on', detail: 'It analyses photos as they come in.' };
  if (woke) return { title: 'Waking your PC…', detail: 'This takes a minute or two.' };
  if (!a.set_up) {
    return {
      title: "Photo analysis isn't set up yet",
      detail: 'Photos wait here, safe, until the analyser runs on your PC.',
    };
  }
  return {
    title: 'Your PC is off',
    detail: `${a.seen ? `Last seen ${ago(a.seen, now)}. ` : ''}Photos wait here until it's back on.`,
  };
}

/** What an item comes to at some grams. */
export function itemTotals(per100: Per100, grams: number): Totals {
  const f = grams / 100;
  const r = (n: number) => Math.round(n * f * 10) / 10;
  return {
    kcal: Math.round(per100.kcal * f),
    protein: r(per100.protein),
    carbs: r(per100.carbs),
    fat: r(per100.fat),
  };
}

/** The draft's calories, rounded to 10 for "about". */
export function draftKcal(items: Pick<DraftItem, 'kcal'>[] | null): number {
  return Math.round((items ?? []).reduce((s, i) => s + i.kcal, 0) / 10) * 10;
}

/** A line about where a photo is. */
export function statusText(p: Pick<MealPhoto, 'status' | 'items'>, online: boolean): string {
  switch (p.status) {
    case 'queued':
      return online ? 'In the queue' : 'Waiting for your PC';
    case 'analysing':
      return 'Analysing…';
    case 'ready':
      return `Ready to check · about ${draftKcal(p.items)} kcal`;
    case 'failed':
      return "Couldn't read it";
    default:
      return 'In your diary';
  }
}

/** Grams to step by: finer for small portions. */
export function gramStep(grams: number): number {
  return grams < 50 ? 5 : grams < 200 ? 10 : 25;
}

/** The SHA-256 of a file, as hex. */
export async function sha256(file: Blob): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', await file.arrayBuffer());
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, '0')).join('');
}

/**
 * A copy of a photo at most `max` pixels on its long side, as JPEG. Drawing it
 * anew leaves the camera's metadata behind, including where it was taken.
 */
export async function shrink(file: Blob, max = 1280): Promise<Blob> {
  const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' });
  const scale = Math.min(1, max / Math.max(bitmap.width, bitmap.height));
  const canvas = document.createElement('canvas');
  canvas.width = Math.round(bitmap.width * scale);
  canvas.height = Math.round(bitmap.height * scale);
  canvas.getContext('2d')!.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
  bitmap.close();
  return new Promise((resolve, reject) =>
    canvas.toBlob(
      (b) => (b ? resolve(b) : reject(new Error("Couldn't read the photo."))),
      'image/jpeg',
      0.85,
    ),
  );
}
