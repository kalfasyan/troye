import type { RecordModel } from 'pocketbase';
import { fastings, weekdays, type Fasting, type Weekday } from './profile';

// Water, exercise, the fasting window and the week's summary.

/** A day's water: one record a day holding the total. */
export interface Water extends RecordModel {
  user: string;
  date: string;
  ml: number;
}

// tracker is the extra calories an activity tracker counted, as Lose It! imports them
export type ExerciseKind =
  | 'walk'
  | 'dog_walk'
  | 'run'
  | 'cycle'
  | 'swim'
  | 'strength'
  | 'yoga'
  | 'hike'
  | 'sport'
  | 'other'
  | 'tracker';

/** An exercise; the server works out kcal from the kind, minutes and weight unless one is given. */
export interface Exercise extends RecordModel {
  user: string;
  date: string;
  kind: ExerciseKind;
  name: string;
  minutes: number;
  kcal: number;
}

// The same METs as backend/app/extras.go, for the estimate while choosing.
export const exerciseKinds: { id: ExerciseKind; label: string; met: number }[] = [
  { id: 'dog_walk', label: 'Dog walk', met: 3.0 },
  { id: 'walk', label: 'Walk', met: 3.5 },
  { id: 'run', label: 'Run', met: 9.8 },
  { id: 'cycle', label: 'Cycling', met: 7.5 },
  { id: 'strength', label: 'Strength', met: 5.0 },
  { id: 'hike', label: 'Hike', met: 6.0 },
  { id: 'swim', label: 'Swim', met: 7.0 },
  { id: 'yoga', label: 'Yoga', met: 2.5 },
  { id: 'sport', label: 'Sport', met: 7.0 },
  { id: 'other', label: 'Other', met: 4.5 },
];

export const kindLabel = (kind: ExerciseKind) =>
  kind === 'tracker'
    ? 'Activity tracker'
    : (exerciseKinds.find((k) => k.id === kind)?.label ?? 'Exercise');

/** The calories an exercise burns above resting: (MET - 1) x kg x hours. */
export function exerciseKcal(kind: ExerciseKind, minutes: number, kg: number): number {
  const met = exerciseKinds.find((k) => k.id === kind)?.met ?? 4.5;
  return Math.round((met - 1) * (kg > 0 ? kg : 70) * (minutes / 60));
}

export const GLASS_ML = 250;
export const WATER_GOAL_ML = 2000;

export interface FastingState {
  eating: boolean;
  /** when the window closes while eating, or opens while fasting */
  until: Date;
  /** how far through the current stretch, 0 to 1 */
  progress: number;
}

/**
 * Where the day is in the fasting plan; null without one, or between fasts
 * when the plan has days off. A fast belongs to the day it starts on.
 */
export function fastingState(
  now: Date,
  start: string,
  fasting: Fasting,
  days: Weekday[] = [],
): FastingState | null {
  const hours = fastings.find((f) => f.id === fasting)?.hours ?? 0;
  if (!hours) return null;
  const [h, m] = start.split(':').map(Number);
  const opens = new Date(now);
  opens.setHours(h, m, 0, 0);
  if (opens > now) opens.setDate(opens.getDate() - 1);
  const window = (24 - hours) * 3600_000;
  const closes = new Date(opens.getTime() + window);
  // the fast that ends this window, or that this is
  const day = weekdays[(closes.getDay() + 6) % 7].id;
  if (days.length && !days.includes(day)) return null;
  if (now < closes) {
    return { eating: true, until: closes, progress: (now.getTime() - opens.getTime()) / window };
  }
  const next = new Date(opens);
  next.setDate(next.getDate() + 1);
  return {
    eating: false,
    until: next,
    progress: (now.getTime() - closes.getTime()) / (next.getTime() - closes.getTime()),
  };
}

/** A stretch of time as "3 h 05 min" or "42 min". */
export function formatDuration(ms: number): string {
  const minutes = Math.max(0, Math.ceil(ms / 60_000));
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return h ? `${h} h ${String(m).padStart(2, '0')} min` : `${m} min`;
}

/** One day of GET /api/app/week. */
export interface WeekDay {
  date: string;
  kcal: number;
  exercise: number;
  /** the calorie budget plus that day's exercise */
  budget: number;
  water_ml: number;
  logged: boolean;
}

export interface Week {
  start: string;
  days: WeekDay[];
  days_logged: number;
  avg_kcal: number;
  under_budget: number;
  weight_change: number;
  has_weight: boolean;
  streak: number;
}
