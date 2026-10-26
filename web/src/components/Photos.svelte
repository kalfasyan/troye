<script lang="ts">
  import { onMount } from 'svelte';
  import { dayLabel, isoDay, meals } from '../lib/diary';
  import { statusText, type MealPhoto } from '../lib/photos';
  import { photos } from '../lib/photos.svelte';
  import AnalyserCard from './AnalyserCard.svelte';
  import LoggedPhoto from './LoggedPhoto.svelte';
  import PhotoReview from './PhotoReview.svelte';

  // Every meal photo, newest first: the ones still waiting, being analysed or
  // ready to check at the top, then the ones in the diary by day. Each opens
  // to check it, or to change or remove the food it was logged as.
  const today = isoDay(new Date());
  const perPage = 60;

  let logged = $state<MealPhoto[]>([]);
  let more = $state(false);
  let loading = $state(true);
  let error = $state('');
  let thumbs = $state<Record<string, string>>({});
  let reviewing = $state<MealPhoto | null>(null);
  let open = $state<MealPhoto | null>(null);

  const days = $derived.by(() => {
    const out: { day: string; list: MealPhoto[] }[] = [];
    for (const p of logged) {
      if (out.at(-1)?.day !== p.date) out.push({ day: p.date, list: [] });
      out.at(-1)!.list.push(p);
    }
    return out;
  });

  onMount(() => {
    photos.load();
    load(1);
    const sub = photos.subscribe();
    return () => {
      sub.then((off) => off()).catch(() => {});
    };
  });

  /** Loads the first n pages again, or the next one. */
  async function load(pages: number, next = false) {
    loading = true;
    try {
      const res = next ? await photos.page(pages, perPage) : await photos.page(1, perPage * pages);
      logged = next ? [...logged, ...res.items] : res.items;
      more = logged.length < res.totalItems;
      error = '';
    } catch {
      error = "Couldn't load your photos.";
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    for (const p of [...photos.pending, ...logged]) {
      if (!thumbs[p.id])
        photos
          .url(p, true)
          .then((u) => (thumbs[p.id] = u))
          .catch(() => {});
    }
  });

  function closed() {
    reviewing = null;
    open = null;
    load(Math.max(1, Math.ceil(logged.length / perPage)));
  }

  const label = (p: MealPhoto) => meals.find((m) => m.id === p.meal)?.label ?? '';
</script>

{#snippet thumb(p: MealPhoto)}
  {#if thumbs[p.id]}
    <img src={thumbs[p.id]} alt="" class="size-full object-cover" />
  {/if}
{/snippet}

<h1 class="text-2xl font-bold tracking-tight">Photos</h1>

<AnalyserCard />

{#if photos.pending.length}
  <section class="card p-0">
    <h2 class="px-4 pt-3 pb-1 text-sm font-medium text-zinc-500 dark:text-zinc-400">
      Not in your diary yet
    </h2>
    <ul class="divide-y divide-zinc-100 dark:divide-zinc-800">
      {#each [...photos.pending].reverse() as p (p.id)}
        <li>
          <button
            class="flex w-full items-center gap-3 px-4 py-2.5 text-left hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
            onclick={() => (reviewing = p)}
          >
            <span class="size-12 shrink-0 overflow-hidden rounded-lg bg-zinc-100 dark:bg-zinc-800"
              >{@render thumb(p)}</span
            >
            <span class="min-w-0 flex-1">
              <span class="block">{label(p)} · {dayLabel(p.date, today)}</span>
              <span
                class="block truncate text-sm {p.status === 'ready'
                  ? 'font-medium text-brand-600 dark:text-brand-400'
                  : p.status === 'failed'
                    ? 'text-rose-600 dark:text-rose-400'
                    : 'text-zinc-500 dark:text-zinc-400'}"
                >{statusText(p, photos.analyser.online)}</span
              >
            </span>
          </button>
        </li>
      {/each}
    </ul>
  </section>
{/if}

{#each days as d (d.day)}
  <section>
    <h2 class="mb-2 text-sm font-medium text-zinc-500 dark:text-zinc-400">
      {dayLabel(d.day, today)}
    </h2>
    <div class="grid grid-cols-3 gap-2">
      {#each d.list as p (p.id)}
        <button
          class="relative aspect-square overflow-hidden rounded-xl bg-zinc-100 dark:bg-zinc-800"
          aria-label="{label(p)} photo, {dayLabel(p.date, today)}"
          onclick={() => (open = p)}
        >
          {@render thumb(p)}
          <span
            class="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/60 px-2 pt-4 pb-1 text-left text-xs font-medium text-white"
            >{label(p)}</span
          >
        </button>
      {/each}
    </div>
  </section>
{:else}
  {#if !loading && !photos.pending.length && !error}
    <section class="card text-center text-sm text-zinc-500 dark:text-zinc-400">
      <img src="/brand/plato.svg" alt="" class="mx-auto mb-2 size-16" />
      No photos yet. Tap <b>+ Add food</b> on a meal in your diary, then <b>Take a photo</b>.
    </section>
  {/if}
{/each}

{#if error}<p class="text-sm text-red-600 dark:text-red-400" role="alert">{error}</p>{/if}
{#if more}
  <button
    class="btn-secondary"
    disabled={loading}
    onclick={() => load(Math.ceil(logged.length / perPage) + 1, true)}
    >{loading ? 'Loading…' : 'Show older photos'}</button
  >
{/if}

{#if reviewing}
  <PhotoReview photo={reviewing} onclose={closed} />
{/if}
{#if open}
  <LoggedPhoto photo={open} onclose={closed} />
{/if}
