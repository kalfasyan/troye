import { diary } from './diary.svelte';
import type { Entry, Meal } from './diary';
import { pb } from './pb';
import { sha256, shrink, type AnalyserStatus, type MealPhoto } from './photos';
import { session } from './session.svelte';

// The person's photos that aren't in the diary yet, the photos of the meals
// of the day the diary shows, and whether the analyser on the PC is on. They
// stay live: a draft shows up the moment it is ready.
class Photos {
  pending = $state<MealPhoto[]>([]);
  /** the logged photos of the day the diary shows */
  logged = $state<MealPhoto[]>([]);
  private loggedDay = '';
  analyser = $state<AnalyserStatus>({
    online: false,
    set_up: true,
    waiting: 0,
    can_wake: false,
    seen: '',
  });
  /** whether the analyser's status has been asked for yet */
  checked = $state(false);
  /** when Wake PC was last tapped, so the cards say it is waking */
  wokeAt = $state(0);
  // one token request shared by every photo on the screen
  private token: Promise<string> | null = null;
  private tokenAt = 0;

  forDay(day: string) {
    return this.pending.filter((p) => p.date === day);
  }

  private put(p: MealPhoto) {
    const rest = this.pending.filter((x) => x.id !== p.id);
    this.pending =
      p.status === 'logged'
        ? rest
        : [...rest, p].sort((a, b) => a.created.localeCompare(b.created));
    this.logged = this.logged.filter((x) => x.id !== p.id);
    if (p.status === 'logged' && p.date === this.loggedDay) this.logged = [...this.logged, p];
  }

  private drop(id: string) {
    this.pending = this.pending.filter((x) => x.id !== id);
    this.logged = this.logged.filter((x) => x.id !== id);
  }

  /** Loads the photos of a day's meals. */
  async showLogged(day: string) {
    this.loggedDay = day;
    try {
      const list = await pb.collection('meal_photos').getFullList<MealPhoto>({
        filter: pb.filter('user = {:user} && date = {:day} && status = "logged"', {
          user: session.user!.id,
          day,
        }),
        sort: 'created',
        requestKey: null,
      });
      if (day === this.loggedDay) this.logged = list;
    } catch {
      // the diary still works without them
      if (day === this.loggedDay) this.logged = [];
    }
  }

  async load() {
    try {
      this.pending = await pb.collection('meal_photos').getFullList<MealPhoto>({
        filter: pb.filter('user = {:user} && status != "logged"', { user: session.user!.id }),
        sort: 'created',
        requestKey: null,
      });
    } catch {
      // the diary still works without them
    }
  }

  async checkAnalyser() {
    try {
      this.analyser = await pb.send<AnalyserStatus>('/api/app/analyser', { requestKey: null });
      this.checked = true;
    } catch {
      // keep the last known status
    }
  }

  /** Shrinks a photo and queues it; a photo already queued is found instead. */
  async add(file: File, meal: Meal, date: string, note: string): Promise<MealPhoto> {
    const hash = await sha256(file);
    const same = this.pending.find((p) => p.hash === hash);
    if (same) return same;
    const small = await shrink(file);
    const form = new FormData();
    form.append('user', session.user!.id);
    form.append('date', date);
    form.append('meal', meal);
    form.append('note', note);
    form.append('hash', hash);
    form.append('status', 'queued');
    form.append('photo', small, 'meal.jpg');
    const p = await pb.collection('meal_photos').create<MealPhoto>(form);
    this.put(p);
    this.checkAnalyser();
    return p;
  }

  /** A page of the person's photos in the diary, newest first, for the Photos tab. */
  async page(n: number, perPage = 60) {
    return pb.collection('meal_photos').getList<MealPhoto>(n, perPage, {
      filter: pb.filter('user = {:user} && status = "logged"', { user: session.user!.id }),
      sort: '-date,-created',
      requestKey: null,
    });
  }

  /**
   * The diary entries a logged photo became. Imported photos don't say which,
   * so for those it is everything logged for that meal that day.
   */
  async entriesOf(p: MealPhoto): Promise<Entry[]> {
    const ids = (p.confirmed ?? []).map((c) => c.entry).filter((id): id is string => !!id);
    const filter = ids.length
      ? ids.map((id) => pb.filter('id = {:id}', { id })).join(' || ')
      : pb.filter('user = {:user} && date = {:date} && meal = {:meal}', {
          user: session.user!.id,
          date: p.date,
          meal: p.meal,
        });
    return pb.collection('entries').getFullList<Entry>({
      filter,
      sort: 'created',
      expand: 'food',
      requestKey: null,
    });
  }

  async get(id: string): Promise<MealPhoto> {
    const p = await pb.collection('meal_photos').getOne<MealPhoto>(id, { requestKey: null });
    if (p.status !== 'logged') this.put(p);
    return p;
  }

  /** Changes the note or meal of a photo that isn't in the diary yet. */
  async edit(p: MealPhoto, fields: { note?: string; meal?: Meal }) {
    this.put(await pb.collection('meal_photos').update<MealPhoto>(p.id, fields));
  }

  /** Sends a photo back to the analyser, with a note to help it. */
  async retry(p: MealPhoto, note: string) {
    this.put(
      await pb.collection('meal_photos').update<MealPhoto>(p.id, { status: 'queued', note }),
    );
  }

  async remove(p: MealPhoto) {
    await pb.collection('meal_photos').delete(p.id);
    this.drop(p.id);
  }

  /** Logs the checked draft as diary entries. */
  async log(
    p: MealPhoto,
    meal: Meal,
    items: { name: string; food: string; grams: number; kcal: number }[],
  ) {
    const done = await pb.send<MealPhoto>(`/api/app/photos/${p.id}/log`, {
      method: 'POST',
      body: { meal, date: p.date, items },
    });
    this.put(done);
    await diary.load();
  }

  async wake() {
    await pb.send('/api/app/analyser/wake', { method: 'POST' });
    this.wokeAt = Date.now();
  }

  /** Whether Wake PC was tapped in the last few minutes. */
  get waking() {
    return !this.analyser.online && Date.now() - this.wokeAt < 5 * 60_000;
  }

  /** A link to a photo; they need a file token, which lasts a few minutes. */
  async url(p: MealPhoto, thumb = false): Promise<string> {
    if (!this.token || Date.now() - this.tokenAt > 60_000) {
      this.tokenAt = Date.now();
      this.token = pb.files.getToken({ requestKey: null });
    }
    try {
      return pb.files.getURL(p, p.photo, {
        token: await this.token,
        thumb: thumb ? '480x480f' : undefined,
      });
    } catch (err) {
      this.token = null;
      throw err;
    }
  }

  async subscribe() {
    return pb.collection('meal_photos').subscribe<MealPhoto>('*', ({ action, record }) => {
      if (action === 'delete') this.drop(record.id);
      else this.put(record);
    });
  }
}

export const photos = new Photos();
