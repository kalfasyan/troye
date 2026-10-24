import { pb } from './pb';
import { profile } from './profile.svelte';
import { session } from './session.svelte';
import { withTrend, type Weighin } from './weight';

// All of the signed-in person's weigh-ins (one a day, so a few hundred a year
// at most), kept live. Each weigh-in can move the daily budget, so the profile
// is reloaded after one changes.
class WeightStore {
  list = $state<Weighin[]>([]);
  loaded = $state(false);
  error = $state('');

  points = $derived(withTrend(this.list));
  latest = $derived(this.points.at(-1) ?? null);

  private put(w: Weighin) {
    this.list = [...this.list.filter((x) => x.id !== w.id), w];
  }

  async load() {
    this.error = '';
    try {
      this.list = await pb.collection('weighins').getFullList<Weighin>({
        filter: pb.filter('user = {:user}', { user: session.user!.id }),
        sort: 'date',
        requestKey: null,
      });
      this.loaded = true;
    } catch {
      this.error = "Couldn't load your weigh-ins.";
    }
  }

  /** Logs a weight for a day, replacing that day's weigh-in if there is one. */
  async log(date: string, kg: number, note = '') {
    const same = this.list.find((w) => w.date === date);
    const w = same
      ? await pb.collection('weighins').update<Weighin>(same.id, { weight_kg: kg, note })
      : await pb
          .collection('weighins')
          .create<Weighin>({ user: session.user!.id, date, weight_kg: kg, note });
    this.put(w);
    await profile.load();
  }

  /** Removes a weigh-in and returns a function that puts it back. */
  async remove(w: Weighin): Promise<() => Promise<void>> {
    await pb.collection('weighins').delete(w.id);
    this.list = this.list.filter((x) => x.id !== w.id);
    await profile.load();
    return () => this.log(w.date, w.weight_kg, w.note);
  }

  /** Live updates from the other device. */
  async subscribe() {
    return pb.collection('weighins').subscribe<Weighin>('*', ({ action, record }) => {
      if (action === 'delete') this.list = this.list.filter((x) => x.id !== record.id);
      else this.put(record);
    });
  }
}

export const weight = new WeightStore();
