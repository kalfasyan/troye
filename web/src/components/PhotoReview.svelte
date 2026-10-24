<script lang="ts">
  import { onMount } from 'svelte';
  import { meals, type Food, type Meal } from '../lib/diary';
  import { searchFoods } from '../lib/diary.svelte';
  import { errorMessage } from '../lib/pb';
  import { gramStep, itemTotals, statusText, type MealPhoto, type Per100 } from '../lib/photos';
  import { photos } from '../lib/photos.svelte';
  import Sheet from './Sheet.svelte';

  // Checking a photo's draft before it goes in the diary: each item with the
  // food it was matched to and its grams, all easy to change. A photo that is
  // still waiting, or couldn't be read, can get a note and go back to the queue.
  let { photo, onclose }: { photo: MealPhoto; onclose: () => void } = $props();

  interface Row {
    name: string;
    food: string;
    food_name: string;
    grams: number;
    per100: Per100;
    confidence: number;
    alternatives: { food: string; name: string; per100: Per100 }[];
  }

  // svelte-ignore state_referenced_locally
  let meal = $state<Meal>(photo.meal);
  // svelte-ignore state_referenced_locally
  let note = $state(photo.note);
  // svelte-ignore state_referenced_locally
  let rows = $state<Row[]>(
    (photo.items ?? []).map((i) => ({
      name: i.name,
      food: i.food,
      food_name: i.food_name,
      grams: i.grams,
      per100: i.per100,
      confidence: i.confidence,
      alternatives: i.alternatives ?? [],
    })),
  );
  let src = $state('');
  let busy = $state(false);
  let error = $state('');

  // picking a food for a row, or for a new one
  let picking = $state<number | 'new' | null>(null);
  let query = $state('');
  let results = $state<Food[]>([]);

  const current = $derived(photos.pending.find((p) => p.id === photo.id) ?? photo);
  const total = $derived(
    rows.reduce((s, r) => s + (r.food ? itemTotals(r.per100, r.grams).kcal : 0), 0),
  );
  const unmatched = $derived(rows.some((r) => !r.food));

  onMount(() => {
    photos.url(photo).then((u) => (src = u));
  });

  // a draft that arrives while this is open fills it in
  $effect(() => {
    if (current.status === 'ready' && !rows.length && current.items?.length) {
      rows = current.items.map((i) => ({ ...i, alternatives: i.alternatives ?? [] }));
    }
  });

  let timer: ReturnType<typeof setTimeout>;
  $effect(() => {
    const q = query;
    clearTimeout(timer);
    if (q.trim().length < 2) {
      results = [];
      return;
    }
    timer = setTimeout(async () => {
      try {
        results = await searchFoods(q);
      } catch {
        // keep the last results
      }
    }, 250);
  });

  function step(i: number, dir: number) {
    const r = rows[i];
    r.grams = Math.max(gramStep(r.grams), r.grams + dir * gramStep(r.grams));
  }

  function choose(
    f: { id?: string; food?: string; name: string; per100?: Per100 } & Partial<Food>,
  ) {
    const per100 = f.per100 ?? { kcal: f.kcal!, protein: f.protein!, carbs: f.carbs!, fat: f.fat! };
    const food = f.food ?? f.id!;
    if (picking === 'new') {
      rows.push({
        name: f.name,
        food,
        food_name: f.name,
        grams: f.serving_g || 100,
        per100,
        confidence: 1,
        alternatives: [],
      });
    } else if (picking !== null) {
      const r = rows[picking];
      // keep the old match as an alternative, so changing back is one tap
      if (r.food && !r.alternatives.some((a) => a.food === r.food)) {
        r.alternatives.unshift({ food: r.food, name: r.food_name, per100: r.per100 });
      }
      r.alternatives = r.alternatives.filter((a) => a.food !== food);
      r.food = food;
      r.food_name = f.name;
      r.per100 = per100;
    }
    picking = null;
    query = '';
  }

  async function log() {
    busy = true;
    error = '';
    try {
      await photos.log(
        current,
        meal,
        rows
          .filter((r) => r.food)
          .map((r) => ({ name: r.name, food: r.food, grams: r.grams, kcal: 0 })),
      );
      onclose();
    } catch (err) {
      error = errorMessage(err, "Couldn't log it. Please try again.");
    } finally {
      busy = false;
    }
  }

  async function retry() {
    busy = true;
    error = '';
    try {
      await photos.retry(current, note.trim());
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  async function remove() {
    busy = true;
    try {
      await photos.remove(current);
      onclose();
    } catch (err) {
      error = errorMessage(err, "Couldn't delete it.");
      busy = false;
    }
  }

  const sure = (c: number) => (c >= 0.75 ? '' : c >= 0.45 ? 'Check this one' : 'Unsure');
</script>

<Sheet
  title={picking !== null ? 'Pick a food' : 'Check your meal'}
  onclose={picking !== null ? () => (picking = null) : onclose}
>
  {#if picking !== null}
    <input
      type="search"
      placeholder="Search foods"
      aria-label="Search foods"
      autocomplete="off"
      bind:value={query}
    />
    {@const row = typeof picking === 'number' ? rows[picking] : null}
    <ul class="mt-3 divide-y divide-zinc-200 dark:divide-zinc-800">
      {#each query.trim().length >= 2 ? results : [] as f (f.id)}
        <li>
          <button
            class="flex w-full justify-between gap-3 py-3 text-left"
            onclick={() => choose(f)}
          >
            <span class="min-w-0 truncate">{f.name}</span>
            <span class="shrink-0 text-sm text-zinc-500 dark:text-zinc-400"
              >{f.kcal} kcal/100 g</span
            >
          </button>
        </li>
      {:else}
        {#if row?.alternatives.length}
          <li class="pb-1 text-sm font-medium text-zinc-500 dark:text-zinc-400">
            Other matches for “{row.name}”
          </li>
        {/if}
        {#each row?.alternatives ?? [] as a (a.food)}
          <li>
            <button
              class="flex w-full justify-between gap-3 py-3 text-left"
              onclick={() => choose(a)}
            >
              <span class="min-w-0 truncate">{a.name}</span>
              <span class="shrink-0 text-sm text-zinc-500 dark:text-zinc-400"
                >{a.per100.kcal} kcal/100 g</span
              >
            </button>
          </li>
        {/each}
      {/each}
    </ul>
  {:else}
    {#if src}
      <img
        {src}
        alt="Your meal"
        class="mb-4 max-h-64 w-full rounded-2xl bg-zinc-100 object-cover dark:bg-zinc-800"
      />
    {/if}

    <div class="mb-4 flex gap-1 overflow-x-auto" role="radiogroup" aria-label="Meal">
      {#each meals as m (m.id)}
        <button
          class="chip shrink-0 py-1.5"
          class:chosen={meal === m.id}
          role="radio"
          aria-checked={meal === m.id}
          onclick={() => (meal = m.id)}>{m.label}</button
        >
      {/each}
    </div>

    {#if current.status === 'ready' || rows.length}
      <ul class="flex flex-col gap-2">
        {#each rows as r, i (i)}
          <li class="card p-4">
            <div class="flex items-start justify-between gap-3">
              <button class="min-w-0 text-left" onclick={() => (picking = i)}>
                <span class="block font-medium {r.food ? '' : 'text-rose-600 dark:text-rose-400'}"
                  >{r.food ? r.food_name : 'Pick a food'}</span
                >
                <span class="block text-sm text-zinc-500 dark:text-zinc-400">
                  Seen as “{r.name}”{sure(r.confidence) ? ` · ${sure(r.confidence)}` : ''}
                  · <span class="text-brand-600 dark:text-brand-400">change</span>
                </span>
              </button>
              <button
                class="-mt-1 -mr-1 rounded-full p-1.5 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800"
                aria-label="Remove {r.food_name || r.name}"
                onclick={() => rows.splice(i, 1)}
              >
                <svg
                  viewBox="0 0 24 24"
                  class="size-4"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"><path d="M6 6l12 12M18 6L6 18" stroke-linecap="round" /></svg
                >
              </button>
            </div>
            <div class="mt-3 flex items-center gap-2">
              <button
                class="btn-secondary size-9 rounded-full p-0"
                aria-label="Less of {r.food_name || r.name}"
                onclick={() => step(i, -1)}>−</button
              >
              <label class="flex-row items-baseline gap-1">
                <span class="sr-only">Grams of {r.food_name || r.name}</span>
                <input
                  class="w-20 text-center tabular-nums"
                  type="number"
                  inputmode="numeric"
                  min="1"
                  max="3000"
                  bind:value={r.grams}
                />
                <span class="font-normal text-zinc-500 dark:text-zinc-400">g</span>
              </label>
              <button
                class="btn-secondary size-9 rounded-full p-0"
                aria-label="More of {r.food_name || r.name}"
                onclick={() => step(i, 1)}>+</button
              >
              <span class="ml-auto font-semibold tabular-nums"
                >{r.food ? `${itemTotals(r.per100, r.grams || 0).kcal} kcal` : '–'}</span
              >
            </div>
          </li>
        {/each}
      </ul>
      <button
        class="mt-2 mb-6 self-start text-sm font-semibold text-brand-600 dark:text-brand-400"
        onclick={() => (picking = 'new')}>+ Add something it missed</button
      >

      {#if error}<p class="mt-3 text-sm text-red-600 dark:text-red-400" role="alert">
          {error}
        </p>{/if}
      {#if unmatched}
        <p class="mt-3 text-sm text-zinc-500 dark:text-zinc-400">
          Items without a food are left out. Tap one to pick its food.
        </p>
      {/if}
      <button
        class="btn-primary mt-auto w-full"
        disabled={busy || !rows.some((r) => r.food)}
        onclick={log}
      >
        {busy
          ? 'Logging…'
          : `Log ${total.toLocaleString()} kcal to ${meals.find((m) => m.id === meal)?.label.toLowerCase()}`}
      </button>
    {:else}
      <div class="card flex items-center gap-4">
        <img
          src={current.status === 'failed' ? '/brand/plato.svg' : '/brand/plato-thinking.svg'}
          alt=""
          class="size-14 shrink-0"
        />
        <div>
          <p class="font-medium">{statusText(current, photos.analyser.online)}</p>
          <p class="text-sm text-zinc-500 dark:text-zinc-400">
            {#if current.status === 'failed'}
              {current.error} Add a note about what's on the plate and try again, or add it by hand.
            {:else if photos.analyser.online}
              Your PC is on it. You'll get a notification when it's ready.
            {:else}
              It's analysed as soon as your PC is on, and you'll get a notification.
            {/if}
          </p>
        </div>
      </div>
      <label class="mt-4"
        >Note for the analyser (optional)
        <input maxlength="200" placeholder="Pasta with pesto, no cheese" bind:value={note} /></label
      >
      {#if error}<p class="mt-3 text-sm text-red-600 dark:text-red-400" role="alert">
          {error}
        </p>{/if}
      <div class="mt-auto grid grid-cols-2 gap-2 pt-4">
        <button
          class="btn-secondary text-red-600 dark:text-red-400"
          disabled={busy}
          onclick={remove}>Delete photo</button
        >
        <button
          class="btn-primary"
          disabled={busy ||
            current.status === 'analysing' ||
            (current.status !== 'failed' && note === current.note)}
          onclick={retry}>{current.status === 'failed' ? 'Try again' : 'Save note'}</button
        >
      </div>
    {/if}
  {/if}
</Sheet>
