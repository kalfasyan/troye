import { pb } from './pb';
import { isoDay, recentFoods, sum, type Entry, type Food, type Meal } from './diary';
import { session } from './session.svelte';

// The diary for one day at a time, kept live: an entry added on the other
// phone, or later by the photo analyser, shows up without a reload.
class Diary {
  day = $state(isoDay(new Date()));
  entries = $state<Entry[]>([]);
  recent = $state<Food[]>([]);
  loading = $state(false);
  error = $state('');

  totals = $derived(sum(this.entries));

  /** Adds or replaces an entry of the day shown; live updates may bring it first. */
  private put(e: Entry) {
    if (e.date !== this.day) return;
    const old = this.entries.find((x) => x.id === e.id);
    if (!e.expand && old?.expand) e.expand = old.expand;
    this.entries = [...this.entries.filter((x) => x.id !== e.id), e].sort((a, b) =>
      a.created.localeCompare(b.created),
    );
  }

  byMeal(meal: Meal) {
    return this.entries.filter((e) => e.meal === meal);
  }

  async show(day: string) {
    this.day = day;
    await this.load();
  }

  async load() {
    const day = this.day;
    this.loading = true;
    this.error = '';
    try {
      const list = await pb.collection('entries').getFullList<Entry>({
        filter: pb.filter('user = {:user} && date = {:day}', { user: session.user!.id, day }),
        sort: 'created',
        expand: 'food',
        requestKey: null,
      });
      if (day === this.day) this.entries = list;
    } catch {
      this.error = "Couldn't load your diary.";
    } finally {
      this.loading = false;
    }
  }

  async loadRecent() {
    try {
      const list = await pb.collection('entries').getList<Entry>(1, 200, {
        filter: pb.filter('user = {:user} && food != ""', { user: session.user!.id }),
        sort: '-created',
        expand: 'food',
        requestKey: null,
      });
      this.recent = recentFoods(list.items);
    } catch {
      // recents are a convenience; search still works
    }
  }

  async add(meal: Meal, food: Food, grams: number) {
    const e = await pb
      .collection('entries')
      .create<Entry>(
        { user: session.user!.id, date: this.day, meal, food: food.id, grams },
        { expand: 'food' },
      );
    this.put(e);
    this.recent = [food, ...this.recent.filter((f) => f.id !== food.id)];
  }

  async quickAdd(meal: Meal, name: string, kcal: number) {
    const e = await pb.collection('entries').create<Entry>({
      user: session.user!.id,
      date: this.day,
      meal,
      name,
      kcal,
    });
    this.put(e);
  }

  async update(entry: Entry, changes: Partial<Pick<Entry, 'grams' | 'meal' | 'kcal' | 'name'>>) {
    const e = await pb.collection('entries').update<Entry>(entry.id, changes, { expand: 'food' });
    this.put(e);
  }

  /** Removes an entry and returns a function that puts it back. */
  async remove(entry: Entry): Promise<() => Promise<void>> {
    await pb.collection('entries').delete(entry.id);
    this.entries = this.entries.filter((e) => e.id !== entry.id);
    return async () => {
      const { user, date, meal, food, name, grams, kcal } = entry;
      const back = await pb
        .collection('entries')
        .create<Entry>({ user, date, meal, food, name, grams, kcal }, { expand: 'food' });
      this.put(back);
    };
  }

  /** Live updates from the other device and the photo analyser. */
  async subscribe() {
    return pb.collection('entries').subscribe<Entry>(
      '*',
      ({ action, record }) => {
        if (action === 'delete') this.entries = this.entries.filter((e) => e.id !== record.id);
        else this.put(record);
      },
      { expand: 'food' },
    );
  }
}

export const diary = new Diary();

/** Finds foods by name or brand: your own first, then the shared tables. */
export async function searchFoods(q: string): Promise<Food[]> {
  const words = q.trim().split(/\s+/).filter(Boolean).slice(0, 4);
  if (!words.length) return [];
  const params: Record<string, string> = {};
  const filter = words
    .map((w, i) => {
      params[`w${i}`] = w;
      return `(name ~ {:w${i}} || brand ~ {:w${i}})`;
    })
    .join(' && ');
  const list = await pb.collection('foods').getList<Food>(1, 30, {
    filter: pb.filter(filter, params),
    sort: '-owner,name',
    requestKey: 'food-search',
  });
  return list.items;
}

/** Looks a barcode up on the server, which asks Open Food Facts the first time. */
export function lookupBarcode(code: string): Promise<Food> {
  return pb.send<Food>(`/api/app/barcode/${encodeURIComponent(code)}`, { requestKey: null });
}

export function saveCustomFood(values: Partial<Food>): Promise<Food> {
  return pb
    .collection('foods')
    .create<Food>({ ...values, source: 'custom', owner: session.user!.id });
}
