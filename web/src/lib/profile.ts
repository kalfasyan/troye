import type { RecordModel } from 'pocketbase';

// A person's plan: their answers to the onboarding interview and the daily
// targets the server works out from them (backend/app/plan.go).

export type Goal = 'lose' | 'maintain' | 'gain';
export type Sex = 'female' | 'male';
export type Activity = 'sedentary' | 'light' | 'moderate' | 'active' | 'very_active';
export type Fasting = 'none' | '12_12' | '14_10' | '16_8' | '18_6' | '20_4' | 'omad';
export type Motivation =
  'health' | 'energy' | 'confidence' | 'fitness' | 'event' | 'mood' | 'family' | 'longevity';
export type Diet = 'balanced' | 'high_protein' | 'low_carb';
export type Units = 'metric' | 'imperial';

/** The answers, as the profiles collection stores them (always metric). */
export interface Answers {
  goal: Goal | '';
  sex: Sex | '';
  birth_date: string;
  height_cm: number;
  weight_kg: number;
  goal_weight_kg: number;
  activity: Activity | '';
  pace_kg: number;
  fasting: Fasting;
  eating_start: string;
  motivations: Motivation[];
  why: string;
  diet: Diet;
  units: Units;
}

export interface Profile extends RecordModel, Answers {
  user: string;
  onboarded: boolean;
  calorie_budget: number;
  protein_g: number;
  carbs_g: number;
  fat_g: number;
  goal_date: string;
}

/** What POST /api/app/plan returns. */
export interface Plan {
  age: number;
  bmi: number;
  bmr: number;
  tdee: number;
  budget: number;
  floored: boolean;
  pace_kg: number;
  protein_g: number;
  carbs_g: number;
  fat_g: number;
  goal_date: string;
}

export const blankAnswers = (): Answers => ({
  goal: '',
  sex: '',
  birth_date: '',
  height_cm: 0,
  weight_kg: 0,
  goal_weight_kg: 0,
  activity: '',
  pace_kg: 0.5,
  fasting: 'none',
  eating_start: '12:00',
  motivations: [],
  why: '',
  diet: 'balanced',
  units: 'metric',
});

export const goals: { id: Goal; label: string; hint: string }[] = [
  { id: 'lose', label: 'Lose weight', hint: 'Eat a little under what you burn' },
  { id: 'maintain', label: 'Stay where I am', hint: 'Eat about what you burn' },
  { id: 'gain', label: 'Gain weight', hint: 'Eat a little over what you burn' },
];

export const motivations: { id: Motivation; label: string; emoji: string }[] = [
  { id: 'health', label: 'Better health', emoji: '❤️' },
  { id: 'energy', label: 'More energy', emoji: '⚡' },
  { id: 'confidence', label: 'Feel confident', emoji: '✨' },
  { id: 'fitness', label: 'Get fitter', emoji: '🏃' },
  { id: 'event', label: 'An event coming up', emoji: '📅' },
  { id: 'mood', label: 'Better mood', emoji: '🌤️' },
  { id: 'family', label: 'For my family', emoji: '👨‍👩‍👧' },
  { id: 'longevity', label: 'Live longer', emoji: '🌱' },
];

export const activities: { id: Activity; label: string; hint: string }[] = [
  { id: 'sedentary', label: 'Mostly sitting', hint: 'A desk job and little exercise' },
  {
    id: 'light',
    label: 'Lightly active',
    hint: 'On your feet some of the day, or exercise 1–3 times a week',
  },
  {
    id: 'moderate',
    label: 'Active',
    hint: 'On your feet most of the day, or exercise 3–5 times a week',
  },
  { id: 'active', label: 'Very active', hint: 'Physical work, or hard exercise 6–7 times a week' },
  {
    id: 'very_active',
    label: 'Athlete',
    hint: 'Training twice a day, or very heavy physical work',
  },
];

export const paces = [0.25, 0.5, 0.75, 1];

export const fastings: { id: Fasting; label: string; hint: string; hours: number }[] = [
  { id: 'none', label: 'No fasting', hint: 'Eat whenever suits you', hours: 0 },
  { id: '12_12', label: '12:12', hint: 'A gentle start: 12 hours fasting', hours: 12 },
  { id: '14_10', label: '14:10', hint: '14 hours fasting, a 10 hour eating window', hours: 14 },
  { id: '16_8', label: '16:8', hint: 'The most popular: an 8 hour eating window', hours: 16 },
  { id: '18_6', label: '18:6', hint: 'A 6 hour eating window', hours: 18 },
  { id: '20_4', label: '20:4', hint: 'A 4 hour eating window', hours: 20 },
  { id: 'omad', label: 'One meal a day', hint: 'About a 1 hour eating window', hours: 23 },
];

export const diets: { id: Diet; label: string; hint: string }[] = [
  { id: 'balanced', label: 'Balanced', hint: '50% carbs, 20% protein, 30% fat' },
  { id: 'high_protein', label: 'High protein', hint: '40% carbs, 30% protein, 30% fat' },
  { id: 'low_carb', label: 'Low carb', hint: '25% carbs, 35% protein, 40% fat' },
];

const KG_PER_LB = 0.45359237;
const CM_PER_IN = 2.54;

export const kgToLb = (kg: number) => kg / KG_PER_LB;
export const lbToKg = (lb: number) => lb * KG_PER_LB;

/** Splits a height into whole feet and inches. */
export function cmToFtIn(cm: number): [number, number] {
  const inches = Math.round(cm / CM_PER_IN);
  return [Math.floor(inches / 12), inches % 12];
}
export const ftInToCm = (ft: number, inches: number) => (ft * 12 + inches) * CM_PER_IN;

/** A weight to show, in the person's units. */
export function formatWeight(kg: number, units: Units): string {
  if (units === 'imperial') return `${Math.round(kgToLb(kg))} lb`;
  return `${Math.round(kg * 10) / 10} kg`;
}

/** A weekly pace to show, in the person's units. */
export function formatPace(kg: number, units: Units): string {
  if (units === 'imperial') return `${Math.round(kgToLb(kg) * 10) / 10} lb a week`;
  return `${kg} kg a week`;
}

/** When the eating window closes, from when it opens and the hours fasting. */
export function eatingEnd(start: string, fasting: Fasting): string {
  const hours = fastings.find((f) => f.id === fasting)?.hours ?? 0;
  const [h, m] = start.split(':').map(Number);
  const end = (h + 24 - hours) % 24;
  return `${String(end).padStart(2, '0')}:${String(m).padStart(2, '0')}`;
}

/** The answers the plan needs, for POST /api/app/plan. */
export function planRequest(a: Answers) {
  return {
    sex: a.sex,
    birth_date: a.birth_date,
    height_cm: a.height_cm,
    weight_kg: a.weight_kg,
    goal: a.goal,
    goal_weight_kg: a.goal === 'maintain' ? a.weight_kg : a.goal_weight_kg,
    activity: a.activity,
    pace_kg: a.goal === 'maintain' ? 0 : a.pace_kg,
    diet: a.diet,
  };
}
