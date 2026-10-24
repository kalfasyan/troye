// Push notifications from this server to this device. The server signs them
// with its own key (GET /api/app/push/key); the service worker shows them.

import { pb } from './pb';
import { isIos } from './platform';
import { pushProblem } from './pushProblem';

export type PushSupport = 'ok' | 'install' | 'insecure' | 'unsupported';

const installed = () =>
  matchMedia('(display-mode: standalone)').matches ||
  (navigator as { standalone?: boolean }).standalone === true;

/** Whether this device can get notifications, and if not, why. */
export function pushSupport(): PushSupport {
  // browsers hide push (and service workers) on plain http
  if (!window.isSecureContext) return 'insecure';
  if ('serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window) {
    return 'ok';
  }
  // iPhones only allow push for the app added to the home screen
  return isIos(navigator.userAgent, navigator.maxTouchPoints) && !installed()
    ? 'install'
    : 'unsupported';
}

/** Brave exposes navigator.brave; it ships with its push service turned off. */
export const isBrave = () => 'brave' in navigator;

function keyBytes(base64url: string): Uint8Array<ArrayBuffer> {
  const b64 = (base64url + '='.repeat((4 - (base64url.length % 4)) % 4))
    .replace(/-/g, '+')
    .replace(/_/g, '/');
  const raw = atob(b64);
  const out = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
  return out;
}

// A browser that can't reach its push service can wait forever; give up instead.
function withTimeout<T>(promise: Promise<T>, ms: number): Promise<T> {
  return Promise.race([
    promise,
    new Promise<never>((_, reject) =>
      setTimeout(() => reject(new Error('no answer from the push service')), ms),
    ),
  ]);
}

async function current(): Promise<PushSubscription | null> {
  if (pushSupport() !== 'ok') return null;
  const reg = await navigator.serviceWorker.getRegistration();
  return (await reg?.pushManager.getSubscription()) ?? null;
}

async function save(sub: PushSubscription) {
  await pb.send('/api/app/push/subscribe', { method: 'POST', body: sub.toJSON() });
}

/** Whether this device gets notifications for the signed-in user. */
export async function pushEnabled(): Promise<boolean> {
  return Notification.permission === 'granted' && !!(await current());
}

export async function enablePush(): Promise<void> {
  const brave = isBrave();
  const permission = await Notification.requestPermission();
  if (permission !== 'granted') {
    throw new Error(
      pushProblem(permission === 'denied' ? 'permission-denied' : 'permission-dismissed', brave),
    );
  }
  const reg = await navigator.serviceWorker.ready;
  let key: string;
  try {
    key = (await pb.send<{ key: string }>('/api/app/push/key', {})).key;
  } catch {
    throw new Error(pushProblem('server', brave));
  }
  let sub: PushSubscription;
  try {
    // start fresh in case the server's key changed (for example a new pb_data)
    await (await reg.pushManager.getSubscription())?.unsubscribe();
    sub = await withTimeout(
      reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: keyBytes(key) }),
      20_000,
    );
  } catch (err) {
    throw new Error(pushProblem('subscribe', brave, (err as Error).message));
  }
  try {
    await save(sub);
  } catch {
    throw new Error(pushProblem('server', brave));
  }
}

export async function disablePush(): Promise<void> {
  const sub = await current();
  if (!sub) return;
  try {
    await pb.send('/api/app/push/unsubscribe', {
      method: 'POST',
      body: { endpoint: sub.endpoint },
    });
  } finally {
    await sub.unsubscribe();
  }
}

export async function testPush(): Promise<void> {
  await pb.send('/api/app/push/test', { method: 'POST' });
}

/** Re-registers this device on start, so the server always knows it, and for whom. */
export async function syncPush(): Promise<void> {
  try {
    const sub = await current();
    if (sub && Notification.permission === 'granted') await save(sub);
  } catch {
    // offline; the next start tries again
  }
}

/** Stops notifications for the user signing out on this device. */
export async function forgetPush(): Promise<void> {
  try {
    await disablePush();
  } catch {
    // offline: the server forgets it once the push service reports it gone
  }
}
