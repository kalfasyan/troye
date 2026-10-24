<script lang="ts">
  import { onMount } from 'svelte';
  import { addDays, dayLabel, isoDay, meals, type Entry, type Meal } from '../lib/diary';
  import { diary } from '../lib/diary.svelte';
  import { errorMessage } from '../lib/pb';
  import { profile } from '../lib/profile.svelte';
  import AddFood from './AddFood.svelte';
  import EditEntry from './EditEntry.svelte';
  import Ring from './Ring.svelte';

  // The day's diary: the calorie ring, macros against their targets, and the
  // meals with what was eaten. The arrows move between days.

  const plan = $derived(profile.current!);
  const today = isoDay(new Date());
  let adding = $state<Meal | null>(null);
  let editing = $state<Entry | null>(null);
  let undo = $state<{ label: string; run: () => Promise<void> } | null>(null);
  let undoTimer: ReturnType<typeof setTimeout>;

  onMount(() => {
    diary.show(today);
    const unsubscribe = diary.subscribe();
    return () => {
      unsubscribe.then((off) => off()).catch(() => {});
    };
  });

  const macros = $derived([
    { label: 'Carbs', eaten: diary.totals.carbs, target: plan.carbs_g },
    { label: 'Protein', eaten: diary.totals.protein, target: plan.protein_g },
    { label: 'Fat', eaten: diary.totals.fat, target: plan.fat_g },
  ]);

  async function removed(entry: Entry) {
    editing = null;
    try {
      const putBack = await diary.remove(entry);
      clearTimeout(undoTimer);
      undo = { label: `Removed ${entry.name}`, run: putBack };
      undoTimer = setTimeout(() => (undo = null), 6000);
    } catch (err) {
      diary.error = errorMessage(err, "Couldn't remove it. Please try again.");
    }
  }

  async function runUndo() {
    const u = undo;
    undo = null;
    try {
      await u?.run();
    } catch (err) {
      diary.error = errorMessage(err, "Couldn't put it back.");
    }
  }
</script>

<header class="flex items-center justify-between">
  <button
    class="rounded-full p-2 text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800"
    aria-label="Previous day"
    onclick={() => diary.show(addDays(diary.day, -1))}
  >
    <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2"
      ><path d="M15 5l-7 7 7 7" stroke-linecap="round" stroke-linejoin="round" /></svg
    >
  </button>
  <h1 class="text-xl font-bold tracking-tight">{dayLabel(diary.day, today)}</h1>
  <button
    class="rounded-full p-2 text-zinc-500 hover:bg-zinc-100 disabled:invisible dark:hover:bg-zinc-800"
    aria-label="Next day"
    disabled={diary.day >= today}
    onclick={() => diary.show(addDays(diary.day, 1))}
  >
    <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2"
      ><path d="M9 5l7 7-7 7" stroke-linecap="round" stroke-linejoin="round" /></svg
    >
  </button>
</header>

{#if diary.error}
  <p
    class="rounded-xl bg-red-50 px-4 py-2 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-300"
    role="alert"
  >
    {diary.error}
  </p>
{/if}

<section class="card">
  <Ring eaten={diary.totals.kcal} budget={plan.calorie_budget} />
  <div class="mt-2 flex justify-center gap-6 text-sm text-zinc-500 dark:text-zinc-400">
    <span
      ><b class="text-zinc-900 tabular-nums dark:text-zinc-100"
        >{Math.round(diary.totals.kcal).toLocaleString()}</b
      > eaten</span
    >
    <span
      ><b class="text-zinc-900 tabular-nums dark:text-zinc-100"
        >{plan.calorie_budget.toLocaleString()}</b
      > budget</span
    >
  </div>
  <div class="mt-5 grid grid-cols-3 gap-3 text-sm">
    {#each macros as m (m.label)}
      <div>
        <span class="block font-medium">{m.label}</span>
        <span class="block text-xs text-zinc-500 tabular-nums dark:text-zinc-400"
          >{Math.round(m.eaten)} / {m.target} g</span
        >
        <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-orange-100 dark:bg-zinc-800">
          <div
            class="h-full rounded-full transition-all {m.eaten > m.target
              ? 'bg-rose-500'
              : 'bg-orange-500'}"
            style="width: {m.target ? Math.min((m.eaten / m.target) * 100, 100) : 0}%"
          ></div>
        </div>
      </div>
    {/each}
  </div>
</section>

{#each meals as m (m.id)}
  {@const items = diary.byMeal(m.id)}
  <section class="card p-0">
    <header class="flex items-center justify-between px-5 pt-4 pb-2">
      <h2 class="font-semibold">{m.label}</h2>
      <span class="text-sm text-zinc-500 tabular-nums dark:text-zinc-400">
        {Math.round(items.reduce((s, e) => s + e.kcal, 0))} kcal
      </span>
    </header>
    {#if items.length}
      <ul class="divide-y divide-zinc-100 dark:divide-zinc-800">
        {#each items as e (e.id)}
          <li>
            <button
              class="flex w-full items-center justify-between gap-3 px-5 py-2.5 text-left hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
              onclick={() => (editing = e)}
            >
              <span class="min-w-0">
                <span class="block truncate">{e.name}</span>
                {#if e.grams}<span class="text-sm text-zinc-500 dark:text-zinc-400"
                    >{e.grams} g</span
                  >{/if}
              </span>
              <span class="shrink-0 text-sm tabular-nums">{Math.round(e.kcal)}</span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
    <button
      class="w-full rounded-b-2xl px-5 py-3 text-left text-sm font-semibold text-brand-600 hover:bg-zinc-50 dark:text-brand-400 dark:hover:bg-zinc-800/50"
      onclick={() => (adding = m.id)}>+ Add food</button
    >
  </section>
{/each}

{#if undo}
  <div
    class="fixed inset-x-4 bottom-24 z-10 mx-auto flex max-w-md items-center justify-between gap-4 rounded-xl bg-zinc-900 px-4 py-3 text-sm text-white shadow-lg dark:bg-zinc-100 dark:text-zinc-900"
    role="status"
  >
    <span class="truncate">{undo.label}</span>
    <button class="font-semibold text-orange-300 dark:text-orange-700" onclick={runUndo}
      >Undo</button
    >
  </div>
{/if}

{#if adding}
  <AddFood meal={adding} onclose={() => (adding = null)} />
{/if}
{#if editing}
  <EditEntry entry={editing} onclose={() => (editing = null)} onremove={removed} />
{/if}
