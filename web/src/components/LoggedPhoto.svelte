<script lang="ts">
  import { dayLabel, entryAmount, isoDay, meals, type Entry } from '../lib/diary';
  import { diary } from '../lib/diary.svelte';
  import { errorMessage } from '../lib/pb';
  import type { MealPhoto } from '../lib/photos';
  import { photos } from '../lib/photos.svelte';
  import EditEntry from './EditEntry.svelte';
  import Sheet from './Sheet.svelte';

  // A photo that is in the diary, with the food it was logged as. Each item
  // can be changed or removed, and the photo deleted with or without its food.
  let { photo, onclose }: { photo: MealPhoto; onclose: () => void } = $props();

  let src = $state('');
  let entries = $state<Entry[]>([]);
  let loaded = $state(false);
  let editing = $state<Entry | null>(null);
  let busy = $state(false);
  let error = $state('');

  const meal = $derived(meals.find((m) => m.id === photo.meal)?.label ?? '');
  const total = $derived(Math.round(entries.reduce((s, e) => s + e.kcal, 0)));

  $effect(() => {
    photos
      .url(photo)
      .then((u) => (src = u))
      .catch(() => {});
    load();
  });

  async function load() {
    try {
      entries = await photos.entriesOf(photo);
    } catch {
      error = "Couldn't load the food logged with this photo.";
    } finally {
      loaded = true;
    }
  }

  async function removed(e: Entry) {
    editing = null;
    try {
      await diary.remove(e);
      entries = entries.filter((x) => x.id !== e.id);
    } catch (err) {
      error = errorMessage(err, "Couldn't remove it. Please try again.");
    }
  }

  async function remove(withFood: boolean) {
    busy = true;
    error = '';
    try {
      if (withFood) for (const e of entries) await diary.remove(e);
      await photos.remove(photo);
      onclose();
    } catch (err) {
      error = errorMessage(err, "Couldn't delete it. Please try again.");
      busy = false;
    }
  }
</script>

<Sheet title="{meal} photo" {onclose}>
  {#if src}
    <img
      {src}
      alt="Your {meal.toLowerCase()}"
      class="mb-3 max-h-80 w-full rounded-2xl bg-zinc-100 object-cover dark:bg-zinc-800"
    />
  {/if}
  <p class="mb-3 text-sm text-zinc-500 dark:text-zinc-400">
    {dayLabel(photo.date, isoDay(new Date()))} · {meal}{entries.length ? ` · ${total} kcal` : ''}
  </p>

  {#if entries.length}
    <ul class="card divide-y divide-zinc-100 p-0 dark:divide-zinc-800">
      {#each entries as e (e.id)}
        <li>
          <button
            class="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
            onclick={() => (editing = e)}
          >
            <span class="min-w-0">
              <span class="block truncate">{e.name}</span>
              {#if entryAmount(e)}<span class="text-sm text-zinc-500 dark:text-zinc-400"
                  >{entryAmount(e)}</span
                >{/if}
            </span>
            <span class="shrink-0 text-sm tabular-nums">{Math.round(e.kcal)}</span>
          </button>
        </li>
      {/each}
    </ul>
    <p class="mt-2 text-xs text-zinc-500 dark:text-zinc-400">Tap an item to change or remove it.</p>
  {:else if loaded}
    <p class="text-sm text-zinc-500 dark:text-zinc-400">
      No food is logged with this photo any more.
    </p>
  {/if}

  {#if error}<p class="mt-3 text-sm text-red-600 dark:text-red-400" role="alert">{error}</p>{/if}

  <div class="mt-auto flex flex-col gap-2 pt-6">
    <button
      class="btn-secondary text-red-600 dark:text-red-400"
      disabled={busy}
      onclick={() => remove(false)}>Delete photo, keep the food</button
    >
    {#if entries.length}
      <button
        class="btn-secondary text-red-600 dark:text-red-400"
        disabled={busy}
        onclick={() => remove(true)}>Delete photo and its food</button
      >
    {/if}
  </div>
</Sheet>

{#if editing}
  <EditEntry entry={editing} onclose={() => ((editing = null), load())} onremove={removed} />
{/if}
