<script lang="ts">
  import { pb } from '../lib/pb';
  import { install } from '../lib/install.svelte';
  import { eatingEnd, fastings, motivations } from '../lib/profile';
  import { profile } from '../lib/profile.svelte';
  import { session } from '../lib/session.svelte';
  import Notifications from './Notifications.svelte';

  const user = $derived(session.user!);
  const plan = $derived(profile.current!);
  const why = $derived(
    plan.why ||
      plan.motivations
        .map((m) => motivations.find((x) => x.id === m)?.label)
        .filter(Boolean)
        .join(' · '),
  );
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

  <section class="card text-center">
    <p class="text-sm text-zinc-500 dark:text-zinc-400">Your daily budget</p>
    <p class="text-5xl font-bold tracking-tight text-brand-600 dark:text-brand-400">
      {plan.calorie_budget.toLocaleString()}
    </p>
    <p class="text-sm text-zinc-500 dark:text-zinc-400">calories</p>
    <div class="mt-4 grid grid-cols-3 gap-2 text-sm">
      <div><b class="block text-lg">{plan.carbs_g} g</b>carbs</div>
      <div><b class="block text-lg">{plan.protein_g} g</b>protein</div>
      <div><b class="block text-lg">{plan.fat_g} g</b>fat</div>
    </div>
    {#if plan.fasting !== 'none'}
      <p class="mt-4 text-sm text-zinc-600 dark:text-zinc-400">
        {fastings.find((f) => f.id === plan.fasting)?.label} fasting: eat from {plan.eating_start}
        to {eatingEnd(plan.eating_start, plan.fasting)}
      </p>
    {/if}
  </section>

  {#if why}
    <section class="card">
      <h2 class="text-sm text-zinc-500 dark:text-zinc-400">Why you're doing this</h2>
      <p class="mt-1 font-medium">{why}</p>
    </section>
  {/if}

  <button class="btn-secondary" onclick={() => profile.redo()}>Change my plan</button>

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
