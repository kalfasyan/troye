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
  confirmed: { name: string; food: string; grams: number; kcal: number }[] | null;
  model: string;
  error: string;
}

export interface AnalyserStatus {
  online: boolean;
  waiting: number;
  can_wake: boolean;
  seen: string;
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
