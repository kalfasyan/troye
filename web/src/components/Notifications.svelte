<script lang="ts">
  import { onMount } from 'svelte';
  import {
    disablePush,
    enablePush,
    isBrave,
    pushEnabled,
    pushSupport,
    testPush,
  } from '../lib/push';

  const support = pushSupport();
  let on = $state(false);
  let busy = $state(false);
  let message = $state('');

  onMount(async () => {
    if (support === 'ok') on = await pushEnabled();
  });

  async function toggle() {
    busy = true;
    message = '';
    try {
      if (on) {
        await disablePush();
        on = false;
      } else {
        await enablePush();
        on = true;
      }
    } catch (err) {
      message = (err as Error).message || 'Could not change notifications.';
    } finally {
      busy = false;
    }
  }

  async function test() {
    message = '';
    try {
      await testPush();
      message = 'Sent. It should arrive in a few seconds.';
    } catch {
      message = 'Could not send a test notification.';
    }
  }
</script>

<section class="card flex flex-col gap-3">
  <h2 class="font-semibold">Notifications</h2>
  {#if support === 'ok'}
    <div class="flex items-center justify-between gap-4">
      <p class="text-sm text-zinc-600 dark:text-zinc-400">
        {on ? 'This device gets notifications.' : 'Get notifications on this device.'}
      </p>
      <button
        class={on ? 'btn-secondary shrink-0' : 'btn-primary shrink-0'}
        onclick={toggle}
        disabled={busy}
      >
        {busy ? '…' : on ? 'Turn off' : 'Turn on'}
      </button>
    </div>
    {#if on}
      <button class="self-start text-sm text-brand-600 dark:text-brand-400" onclick={test}>
        Send a test notification
      </button>
    {:else if isBrave()}
      <p class="text-sm text-zinc-500">
        In Brave, first open brave://settings/privacy, turn on “Use Google services for push
        messaging” and restart Brave.
      </p>
    {/if}
  {:else if support === 'insecure'}
    <p class="text-sm text-zinc-500">
      Notifications only work over HTTPS. Open this app through its https:// address and turn them
      on there.
    </p>
  {:else if support === 'install'}
    <p class="text-sm text-zinc-500">
      On iPhone, notifications work once the app is on your home screen: tap Share, then Add to Home
      Screen, and open it from there.
    </p>
  {:else}
    <p class="text-sm text-zinc-500">This browser can't show notifications.</p>
  {/if}
  {#if message}<p class="text-sm text-zinc-500" role="status">{message}</p>{/if}
</section>
