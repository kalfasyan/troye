<script lang="ts">
  import { pb } from '../lib/pb';
  import { install } from '../lib/install.svelte';
  import { session } from '../lib/session.svelte';
  import Notifications from './Notifications.svelte';

  const user = $derived(session.user!);
  const version = pb
    .send<{ version: string }>('/api/app/info', {})
    .then((i) => i.version)
    .catch(() => '');
</script>

<main
  class="mx-auto flex min-h-dvh max-w-md flex-col gap-4 px-4 pt-[max(1.5rem,env(safe-area-inset-top))] pb-8"
>
  <header class="flex items-center justify-between">
    <h1 class="text-2xl font-bold tracking-tight">Hi {user.name || user.email}</h1>
    <button class="btn-secondary px-3 py-2 text-sm" onclick={() => session.logout()}
      >Sign out</button
    >
  </header>

  <section class="card">
    <h2 class="font-semibold">Your app starts here</h2>
    <p class="mt-1 text-zinc-600 dark:text-zinc-400">
      Edit <code class="font-mono text-sm">web/src/components/Home.svelte</code> and save.
    </p>
  </section>

  {#if install.canPrompt}
    <section class="card flex items-center justify-between gap-4">
      <p>Install this app for quick access from your home screen.</p>
      <button class="btn-primary shrink-0" onclick={() => install.install()}>Install</button>
    </section>
  {:else if install.needsIosSteps}
    <section class="card">
      <p>To install this app, tap <b>Share</b> in Safari, then <b>Add to Home Screen</b>.</p>
    </section>
  {/if}

  <Notifications />

  <section class="card text-sm text-zinc-600 dark:text-zinc-400">
    Signed in as {user.email}{session.isOwner ? ' (owner)' : ''}.
    {#if session.isOwner}
      Manage users and data in the <a
        class="text-brand-600 underline dark:text-brand-400"
        href="/_/">admin page</a
      >.
    {/if}
  </section>

  {#await version then v}
    {#if v}<p class="mt-auto text-center text-xs text-zinc-400">Version {v}</p>{/if}
  {/await}
</main>
