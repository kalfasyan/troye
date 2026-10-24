<script lang="ts">
  import { onMount } from 'svelte';
  import { meals } from '../lib/diary';
  import { errorMessage } from '../lib/pb';
  import { statusText, type MealPhoto } from '../lib/photos';
  import { photos } from '../lib/photos.svelte';

  // The day's photos that aren't in the diary yet, and whether the PC that
  // analyses them is on, with a button to wake it.
  let { day, onopen }: { day: string; onopen: (p: MealPhoto) => void } = $props();

  const list = $derived(photos.forDay(day));
  const waiting = $derived(
    photos.pending.filter((p) => p.status === 'queued' || p.status === 'analysing'),
  );
  let thumbs = $state<Record<string, string>>({});
  let waking = $state(false);
  let woke = $state(false);
  let error = $state('');

  onMount(() => {
    photos.checkAnalyser();
    // check on the PC while photos wait for it
    const every = setInterval(() => {
      if (waiting.length || woke) photos.checkAnalyser();
    }, 30_000);
    return () => clearInterval(every);
  });

  $effect(() => {
    for (const p of list) {
      if (!thumbs[p.id]) photos.url(p, true).then((u) => (thumbs[p.id] = u));
    }
  });

  async function wake() {
    waking = true;
    error = '';
    try {
      await photos.wake();
      woke = true;
    } catch (err) {
      error = errorMessage(err, "Couldn't wake your PC.");
    } finally {
      waking = false;
    }
  }
</script>

{#if list.length}
  <section class="card p-0">
    <ul class="divide-y divide-zinc-100 dark:divide-zinc-800">
      {#each list as p (p.id)}
        <li>
          <button
            class="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
            onclick={() => onopen(p)}
          >
            {#if thumbs[p.id]}
              <img src={thumbs[p.id]} alt="" class="size-12 shrink-0 rounded-xl object-cover" />
            {:else}
              <span class="size-12 shrink-0 rounded-xl bg-zinc-100 dark:bg-zinc-800"></span>
            {/if}
            <span class="min-w-0 flex-1">
              <span class="block font-medium"
                >{meals.find((m) => m.id === p.meal)?.label} photo</span
              >
              <span
                class="block truncate text-sm {p.status === 'ready'
                  ? 'font-medium text-brand-600 dark:text-brand-400'
                  : p.status === 'failed'
                    ? 'text-rose-600 dark:text-rose-400'
                    : 'text-zinc-500 dark:text-zinc-400'}"
                >{statusText(p, photos.analyser.online)}</span
              >
            </span>
            {#if p.status === 'analysing'}
              <span
                class="size-4 shrink-0 animate-spin rounded-full border-2 border-orange-200 border-t-orange-500"
                aria-hidden="true"
              ></span>
            {/if}
          </button>
        </li>
      {/each}
    </ul>
    {#if waiting.length}
      <div
        class="flex items-center justify-between gap-3 border-t border-zinc-100 px-4 py-3 text-sm dark:border-zinc-800"
      >
        <span class="flex items-center gap-2 text-zinc-600 dark:text-zinc-400">
          <span
            class="size-2 rounded-full {photos.analyser.online ? 'bg-emerald-500' : 'bg-zinc-400'}"
            aria-hidden="true"
          ></span>
          {photos.analyser.online
            ? 'Your PC is on'
            : woke
              ? 'Waking your PC… this takes a minute'
              : 'Your PC is off'}
        </span>
        {#if !photos.analyser.online && photos.analyser.can_wake}
          <button
            class="font-semibold text-brand-600 dark:text-brand-400"
            disabled={waking}
            onclick={wake}>{woke ? 'Wake again' : 'Wake PC'}</button
          >
        {/if}
      </div>
    {/if}
    {#if error}<p class="px-4 pb-3 text-sm text-red-600 dark:text-red-400" role="alert">
        {error}
      </p>{/if}
  </section>
{/if}
