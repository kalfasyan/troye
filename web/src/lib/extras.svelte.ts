import { pb } from './pb';
import { session } from './session.svelte';
import type { Exercise, ExerciseKind, Water, Week } from './extras';

// The water and exercise of the day the diary shows, kept live like the diary.
class Extras {
  day = '';
  water = $state<Water | null>(null);
  exercises = $state<Exercise[]>([]);

  burned = $derived(this.exercises.reduce((s, x) => s + x.kcal, 0));
  ml = $derived(this.water?.ml ?? 0);

  private put(x: Exercise) {
    if (x.date !== this.day) return;
    this.exercises = [...this.exercises.filter((e) => e.id !== x.id), x].sort((a, b) =>
      a.created.localeCompare(b.created),
    );
  }

  async show(day: string) {
    this.day = day;
    const filter = pb.filter('user = {:user} && date = {:day}', { user: session.user!.id, day });
    const [water, exercises] = await Promise.all([
      pb.collection('water').getList<Water>(1, 1, { filter, requestKey: null }),
      pb
        .collection('exercises')
        .getFullList<Exercise>({ filter, sort: 'created', requestKey: null }),
    ]);
    if (day !== this.day) return;
    this.water = water.items[0] ?? null;
    this.exercises = exercises;
  }

  /** Sets the day's water, never below zero. */
  async setWater(ml: number) {
    ml = Math.max(0, Math.min(ml, 20000));
    const before = this.water;
    // show it straight away; taps come quickly
    if (before) this.water = { ...before, ml };
    try {
      this.water = before
        ? await pb.collection('water').update<Water>(before.id, { ml }, { requestKey: null })
        : await pb
            .collection('water')
            .create<Water>({ user: session.user!.id, date: this.day, ml }, { requestKey: null });
    } catch (err) {
      this.water = before;
      throw err;
    }
  }

  async addExercise(kind: ExerciseKind, minutes: number, name = '', kcal = 0) {
    const x = await pb
      .collection('exercises')
      .create<Exercise>({ user: session.user!.id, date: this.day, kind, minutes, name, kcal });
    this.put(x);
  }

  async updateExercise(
    x: Exercise,
    changes: Partial<Pick<Exercise, 'kind' | 'minutes' | 'name' | 'kcal'>>,
  ) {
    this.put(await pb.collection('exercises').update<Exercise>(x.id, changes));
  }

  /** Removes an exercise and returns a function that puts it back. */
  async removeExercise(x: Exercise): Promise<() => Promise<void>> {
    await pb.collection('exercises').delete(x.id);
    this.exercises = this.exercises.filter((e) => e.id !== x.id);
    return () => this.addExercise(x.kind, x.minutes, x.name, x.kcal);
  }

  /** Live updates from the other device. */
  async subscribe() {
    const offs = await Promise.all([
      pb.collection('exercises').subscribe<Exercise>('*', ({ action, record }) => {
        if (action === 'delete') this.exercises = this.exercises.filter((e) => e.id !== record.id);
        else this.put(record);
      }),
      pb.collection('water').subscribe<Water>('*', ({ action, record }) => {
        if (record.date !== this.day) return;
        this.water = action === 'delete' ? null : record;
      }),
    ]);
    return async () => {
      for (const off of offs) await off();
    };
  }
}

export const extras = new Extras();

/** The week from its Monday that a day is in. */
export function loadWeek(day: string): Promise<Week> {
  return pb.send<Week>(`/api/app/week?day=${day}`, { requestKey: null });
}
