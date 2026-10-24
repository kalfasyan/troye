// Plain-language reasons why turning notifications on failed, kept apart
// from push.ts so they can be tested without a browser.

export type PushStep = 'permission-denied' | 'permission-dismissed' | 'subscribe' | 'server';

/** What to tell the person when turning notifications on fails at `step`. */
export function pushProblem(step: PushStep, brave: boolean, detail = ''): string {
  switch (step) {
    case 'permission-denied':
      return brave
        ? 'Brave is blocking notifications for this site. In Brave, open Settings > Site settings > Notifications, remove this address from Blocked (or set it to Allow), then try again.'
        : 'Notifications are blocked for this site. In your browser, open the site settings for this address, allow Notifications, then try again.';
    case 'permission-dismissed':
      return 'The permission prompt closed without an answer. Tap Turn on again and choose Allow.';
    case 'subscribe':
      return brave
        ? 'Brave has its push service turned off. Open brave://settings/privacy in Brave, turn on “Use Google services for push messaging”, restart Brave, then try again.'
        : `This browser could not register for push messages${detail ? ` (${detail})` : ''}. Check that it is up to date and allowed to use the internet in the background.`;
    case 'server':
      return 'Could not reach the server to save this device. Try again in a moment.';
  }
}
