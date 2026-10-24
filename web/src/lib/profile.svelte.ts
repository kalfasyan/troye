import type { FoodLanguage } from './diary';
import { pb } from './pb';
import type { Answers, Plan, Profile } from './profile';
import { planRequest } from './profile';
import { session } from './session.svelte';

// The signed-in person's profile. It is loaded once after sign-in; until it
// exists and is marked onboarded, the app shows the onboarding interview.
class ProfileStore {
  current = $state<Profile | null>(null);
  loaded = $state(false);
  error = $state('');
  private user = session.user?.id;

  constructor() {
    // someone else signing in on this device gets their own profile
    pb.authStore.onChange(() => {
      const user = pb.authStore.record?.id;
      if (user === this.user) return;
      this.user = user;
      this.current = null;
      this.loaded = false;
    });
  }

  get onboarded() {
    return this.current?.onboarded === true;
  }

  private loading: Promise<void> | null = null;

  /** Loads the profile; calls while it is loading share the one request. */
  load() {
    this.loading ??= this.fetch().finally(() => (this.loading = null));
    return this.loading;
  }

  private async fetch() {
    this.error = '';
    try {
      const list = await pb.collection('profiles').getList<Profile>(1, 1, {
        filter: pb.filter('user = {:user}', { user: session.user!.id }),
        requestKey: null,
      });
      this.current = list.items[0] ?? null;
      this.loaded = true;
    } catch {
      this.error = "Couldn't load your plan.";
    }
  }

  /** Saves the answers; the server works out the daily targets. */
  async save(answers: Answers, onboarded = true) {
    const body = { ...answers, ...planRequest(answers), onboarded };
    this.current = this.current
      ? await pb.collection('profiles').update<Profile>(this.current.id, body)
      : await pb.collection('profiles').create<Profile>({ ...body, user: session.user!.id });
  }

  /** Sets the language food names show in. */
  async setFoodLanguage(food_language: FoodLanguage) {
    if (this.current) {
      this.current = await pb
        .collection('profiles')
        .update<Profile>(this.current.id, { food_language });
    }
  }

  /** Opens the interview again, keeping the answers. */
  async redo() {
    if (this.current) {
      this.current = await pb
        .collection('profiles')
        .update<Profile>(this.current.id, { onboarded: false });
    }
  }
}

export const profile = new ProfileStore();

/** Previews the plan for some answers; throws with a sentence to show. */
export function previewPlan(answers: Answers): Promise<Plan> {
  return pb.send<Plan>('/api/app/plan', { method: 'POST', body: planRequest(answers) });
}
