import PocketBase, { type RecordModel } from 'pocketbase';

// The web app is served by the backend, so the API is on the same origin.
export const pb = new PocketBase('/');

export interface User extends RecordModel {
  email: string;
  name: string;
  role: 'owner' | 'member' | '';
}

/** Turns an API error into a sentence to show the user. */
export function errorMessage(
  err: unknown,
  fallback = 'Something went wrong. Please try again.',
): string {
  const e = err as { status?: number; response?: { message?: string } };
  if (e?.status === 0) return "Can't reach the server. Check your connection.";
  return e?.response?.message || fallback;
}
