import { pb, type User } from './pb';
import { forgetPush, syncPush } from './push';

// Who is signed in. PocketBase keeps the token in localStorage, so a reload or
// reopening the app keeps you signed in.
class Session {
  user = $state<User | null>(current());

  constructor() {
    pb.authStore.onChange(() => (this.user = current()));
  }

  get isOwner() {
    return this.user?.role === 'owner';
  }

  async login(email: string, password: string) {
    await pb.collection('users').authWithPassword(email.trim().toLowerCase(), password);
  }

  /** Signs out, and stops this device's notifications for the user. */
  async logout() {
    await forgetPush();
    pb.authStore.clear();
  }

  /** Checks the saved token is still valid and refreshes the user's details. */
  async refresh() {
    if (!pb.authStore.isValid) return pb.authStore.clear();
    try {
      await pb.collection('users').authRefresh();
      await syncPush();
    } catch (err) {
      const status = (err as { status?: number }).status;
      if (status === 401 || status === 403 || status === 404) pb.authStore.clear();
    }
  }
}

function current(): User | null {
  return pb.authStore.isValid ? (pb.authStore.record as User) : null;
}

export const session = new Session();
