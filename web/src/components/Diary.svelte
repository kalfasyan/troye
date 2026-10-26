<script lang="ts">
  import { onMount } from 'svelte';
  import {
    addDays,
    dayLabel,
    entryAmount,
    isoDay,
    meals,
    type Entry,
    type Meal,
  } from '../lib/diary';
  import { diary } from '../lib/diary.svelte';
  import { kindLabel, type Exercise } from '../lib/extras';
  import { extras } from '../lib/extras.svelte';
  import { errorMessage } from '../lib/pb';
  import type { MealPhoto } from '../lib/photos';
  import { photos } from '../lib/photos.svelte';
  import { profile } from '../lib/profile.svelte';
  import AddExercise from './AddExercise.svelte';
  import AddFood from './AddFood.svelte';
  import EditEntry from './EditEntry.svelte';
  import FastingCard from './FastingCard.svelte';
  import MealPhotos from './MealPhotos.svelte';
  import PhotoReview from './PhotoReview.svelte';
  import PhotoStrip from './PhotoStrip.svelte';
  import Ring from './Ring.svelte';
  import WaterCard from './WaterCard.svelte';
  import WeekSheet from './Week.svelte';

  // The day's diary: the calorie ring, macros against their targets, the
  // fasting window, the meals with what was eaten, exercise and water. The
  // arrows move between days.

  const plan = $derived(profile.current!);
  const today = isoDay(new Date());
  let adding = $state<Meal | null>(null);
  let editing = $state<Entry | null>(null);
  let reviewing = $state<MealPhoto | null>(null);
  let exercising = $state<Exercise | 'new' | null>(null);
  let showWeek = $state(false);
  let undo = $state<{ label: string; run: () => Promise<void> } | null>(null);
  let undoTimer: ReturnType<typeof setTimeout>;

  onMount(() => {
    diary.show(today);
    photos.load();
    openFromHash();
    addEventListener('hashchange', openFromHash);
    const subscriptions = [diary.subscribe(), photos.subscribe(), extras.subscribe()];
    return () => {
      removeEventListener('hashchange', openFromHash);
      for (const s of subscriptions) s.then((off) => off()).catch(() => {});
    };
  });

  $effect(() => {
    extras.show(diary.day).catch(() => (diary.error = "Couldn't load your water and exercise."));
    photos.showLogged(diary.day);
  });

  // exercise adds to the day's budget
  const budget = $derived(plan.calorie_budget + extras.burned);

  // a notification about a photo opens #photo/<id>, the Monday one #week
  async function openFromHash() {
    if (location.hash === '#week') {
      history.replaceState(null, '', '#diary');
      showWeek = true;
      return;
    }
    const id = location.hash.match(/^#photo\/(\w+)$/)?.[1];
    if (!id) return;
    history.replaceState(null, '', '#diary');
    try {
      const p = await photos.get(id);
      if (p.status === 'logged') {
        diary.show(p.date);
      } else {
        if (p.date !== diary.day) diary.show(p.date);
        reviewing = p;
      }
    } catch {
      diary.error = "That photo isn't here any more.";
    }
  }

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

  async function removedExercise(x: Exercise) {
    exercising = null;
    try {
      const putBack = await extras.removeExercise(x);
      clearTimeout(undoTimer);
      undo = { label: `Removed ${kindLabel(x.kind).toLowerCase()}`, run: putBack };
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
  <Ring eaten={diary.totals.kcal} {budget} />
  <div class="mt-2 flex justify-center gap-6 text-sm text-zinc-500 dark:text-zinc-400">
    <span
      ><b class="text-zinc-900 tabular-nums dark:text-zinc-100"
        >{Math.round(diary.totals.kcal).toLocaleString()}</b
      > eaten</span
    >
    <span
      ><b class="text-zinc-900 tabular-nums dark:text-zinc-100">{budget.toLocaleString()}</b> budget</span
    >
    {#if extras.burned}
      <span
        ><b class="text-zinc-900 tabular-nums dark:text-zinc-100"
          >{extras.burned.toLocaleString()}</b
        > exercise</span
      >
    {/if}
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

{#if diary.day === today && plan.fasting !== 'none'}
  <FastingCard start={plan.eating_start} fasting={plan.fasting} days={plan.fasting_days} />
{/if}

<PhotoStrip day={diary.day} onopen={(p) => (reviewing = p)} />

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
                {#if entryAmount(e)}<span class="text-sm text-zinc-500 dark:text-zinc-400"
                    >{entryAmount(e)}</span
                  >{/if}
              </span>
              <span class="shrink-0 text-sm tabular-nums">{Math.round(e.kcal)}</span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
    <MealPhotos
      list={photos.logged.filter((p) => p.meal === m.id)}
      title={m.label}
      onerror={(msg) => (diary.error = msg)}
    />
    <button
      class="w-full rounded-b-2xl px-5 py-3 text-left text-sm font-semibold text-brand-600 hover:bg-zinc-50 dark:text-brand-400 dark:hover:bg-zinc-800/50"
      onclick={() => (adding = m.id)}>+ Add food</button
    >
  </section>
{/each}

<section class="card p-0">
  <header class="flex items-center justify-between px-5 pt-4 pb-2">
    <h2 class="font-semibold">Exercise</h2>
    <span class="text-sm text-zinc-500 tabular-nums dark:text-zinc-400">
      {extras.burned ? `+${extras.burned}` : 0} kcal
    </span>
  </header>
  {#if extras.exercises.length}
    <ul class="divide-y divide-zinc-100 dark:divide-zinc-800">
      {#each extras.exercises as x (x.id)}
        <li>
          <button
            class="flex w-full items-center justify-between gap-3 px-5 py-2.5 text-left hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
            onclick={() => (exercising = x)}
          >
            <span class="min-w-0">
              <span class="block truncate">{kindLabel(x.kind)}</span>
              <span class="block truncate text-sm text-zinc-500 dark:text-zinc-400"
                >{[x.minutes ? `${x.minutes} min` : '', x.name].filter(Boolean).join(' · ')}</span
              >
            </span>
            <span class="shrink-0 text-sm tabular-nums">{x.kcal}</span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
  <button
    class="w-full rounded-b-2xl px-5 py-3 text-left text-sm font-semibold text-brand-600 hover:bg-zinc-50 dark:text-brand-400 dark:hover:bg-zinc-800/50"
    onclick={() => (exercising = 'new')}>+ Add exercise</button
  >
</section>

<WaterCard onerror={(m) => (diary.error = m)} />

<button class="btn-secondary" onclick={() => (showWeek = true)}>See your week</button>

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
{#if reviewing}
  <PhotoReview photo={reviewing} onclose={() => (reviewing = null)} />
{/if}
{#if exercising}
  <AddExercise
    editing={exercising === 'new' ? null : exercising}
    onclose={() => (exercising = null)}
    onremove={removedExercise}
  />
{/if}
{#if showWeek}
  <WeekSheet units={plan.units} onclose={() => (showWeek = false)} />
{/if}
{#if editing}
  <EditEntry entry={editing} onclose={() => (editing = null)} onremove={removed} />
{/if}
