import { pb } from './pb';
import { isoDay, recentFoods, sum, type Entry, type Food, type Meal } from './diary';
import { session } from './session.svelte';

// The diary for one day at a time, kept live: an entry added on the other
// phone, or later by the photo analyser, shows up without a reload.
class Diary {
  day = $state(isoDay(new Date()));
  entries = $state<Entry[]>([]);
  recent = $state<Food[]>([]);
  /** how much of each recent food was logged last time, to start from next time */
  lastAmounts = new Map<string, { grams: number; amount: string }>();
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

  /** A meal's entries, without the add-ons, which go with the entry they belong to. */
  byMeal(meal: Meal) {
    const ids = new Set(this.entries.map((e) => e.id));
    return this.entries.filter((e) => e.meal === meal && !(e.parent && ids.has(e.parent)));
  }

  /** What was added to an entry, like the milk in a coffee. */
  addOnsOf(entry: Entry) {
    return this.entries.filter((e) => e.parent === entry.id);
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
      // newest first, so the first one seen is the last logged
      for (const e of [...list.items].reverse()) {
        this.lastAmounts.set(e.food, { grams: e.grams, amount: e.amount ?? '' });
      }
    } catch {
      // recents are a convenience; search still works
    }
  }

  /** Logs a food, to a meal or as an add-on to another entry (the server puts it on that entry's meal). */
  async add(meal: Meal, food: Food, grams: number, amount = '', parent?: Entry) {
    const e = await pb.collection('entries').create<Entry>(
      {
        user: session.user!.id,
        date: this.day,
        meal,
        food: food.id,
        grams,
        amount,
        parent: parent?.id,
      },
      { expand: 'food' },
    );
    this.put(e);
    this.recent = [food, ...this.recent.filter((f) => f.id !== food.id)];
    this.lastAmounts.set(food.id, { grams, amount });
    return e;
  }

  async quickAdd(meal: Meal, name: string, kcal: number, parent?: Entry) {
    const e = await pb.collection('entries').create<Entry>({
      user: session.user!.id,
      date: this.day,
      meal,
      name,
      kcal,
      parent: parent?.id,
    });
    this.put(e);
    return e;
  }

  async update(
    entry: Entry,
    changes: Partial<
      Pick<Entry, 'grams' | 'meal' | 'kcal' | 'name' | 'protein' | 'carbs' | 'fat' | 'amount'>
    >,
  ) {
    const e = await pb.collection('entries').update<Entry>(entry.id, changes, { expand: 'food' });
    this.put(e);
    // the server moves the add-ons with it; live updates bring them too, but not always first
    for (const a of this.addOnsOf(e)) {
      if (a.meal !== e.meal || a.date !== e.date) this.put({ ...a, meal: e.meal, date: e.date });
    }
    if (e.date !== this.day)
      this.entries = this.entries.filter((x) => x.id !== e.id && x.parent !== e.id);
  }

  /** Removes an entry, and its add-ons with it, and returns a function that puts them back. */
  async remove(entry: Entry): Promise<() => Promise<void>> {
    const addOns = this.addOnsOf(entry);
    await pb.collection('entries').delete(entry.id);
    this.entries = this.entries.filter((e) => e.id !== entry.id && e.parent !== entry.id);
    const values = (e: Entry) => {
      const { id, created, updated, collectionId, collectionName, expand, ...rest } = e;
      return rest;
    };
    return async () => {
      const back = await pb.collection('entries').create<Entry>(values(entry), { expand: 'food' });
      this.put(back);
      for (const a of addOns) {
        const again = await pb
          .collection('entries')
          .create<Entry>({ ...values(a), parent: back.id }, { expand: 'food' });
        this.put(again);
      }
    };
  }

  /** What this person has added to a food before, most used first, to offer again. */
  async usualAddOns(food: string): Promise<Food[]> {
    try {
      const list = await pb.collection('entries').getList<Entry>(1, 100, {
        filter: pb.filter('user = {:user} && parent.food = {:food} && food != ""', {
          user: session.user!.id,
          food,
        }),
        sort: '-created',
        expand: 'food',
        requestKey: null,
      });
      const count = new Map<string, number>();
      for (const e of list.items) count.set(e.food, (count.get(e.food) ?? 0) + 1);
      return recentFoods(list.items).sort((a, b) => count.get(b.id)! - count.get(a.id)!);
    } catch {
      return [];
    }
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

/**
 * Finds foods by any of their names (English, Dutch or French) or brand, best
 * first: what you log often, your own foods, then the food tables and products.
 */
export function searchFoods(q: string): Promise<Food[]> {
  if (!q.trim()) return Promise.resolve([]);
  return pb.send<Food[]>('/api/app/foods/search', {
    query: { q, limit: 30 },
    requestKey: 'food-search',
  });
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
