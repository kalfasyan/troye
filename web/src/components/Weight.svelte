<script lang="ts">
  import { onMount } from 'svelte';
  import { isoDay } from '../lib/diary';
  import { errorMessage } from '../lib/pb';
  import { formatPace, formatWeight } from '../lib/profile';
  import { profile } from '../lib/profile.svelte';
  import {
    daysBetween,
    formatChange,
    progress,
    projectedDate,
    ranges,
    shortDate,
    weeklyRate,
    type Range,
    type Weighin,
  } from '../lib/weight';
  import { weight } from '../lib/weight.svelte';
  import LogWeight from './LogWeight.svelte';
  import WeightChart from './WeightChart.svelte';

  // Weight over time: where you are against the goal, the trend chart, how
  // fast it is going, and every weigh-in.
  const plan = $derived(profile.current!);
  const units = $derived(plan.units);
  const today = isoDay(new Date());
  const hasGoal = $derived(plan.goal === 'lose' || plan.goal === 'gain');

  let logging = $state(false);
  let editing = $state<Weighin | null>(null);
  let range = $state<Range>('3m');
  let allShown = $state(false);
  let undo = $state<{ label: string; run: () => Promise<void> } | null>(null);
  let undoTimer: ReturnType<typeof setTimeout>;

  onMount(() => {
    weight.load();
    const unsubscribe = weight.subscribe();
    return () => {
      unsubscribe.then((off) => off()).catch(() => {});
    };
  });

  const points = $derived(weight.points);
  const start = $derived(points[0] ?? null);
  const now = $derived(weight.latest);
  const shown = $derived.by(() => {
    const days = ranges.find((r) => r.id === range)!.days;
    return points.filter((p) => daysBetween(p.date, today) <= days);
  });

  const reached = $derived(
    now !== null &&
      hasGoal &&
      (plan.goal === 'lose' ? now.kg <= plan.goal_weight_kg : now.kg >= plan.goal_weight_kg),
  );
  const rate = $derived(weeklyRate(points, today));
  const eta = $derived(
    now && hasGoal && !reached ? projectedDate(now.trend, plan.goal_weight_kg, rate, today) : null,
  );
  const done = $derived(
    start && now && hasGoal ? progress(start.kg, now.kg, plan.goal_weight_kg) : 0,
  );

  // the weights that give a BMI of 18.5 to 24.9 at this height
  const healthy = $derived.by(() => {
    const m = plan.height_cm / 100;
    return [18.5 * m * m, 24.9 * m * m];
  });
  const bmi = $derived(now ? Math.round((now.kg / (plan.height_cm / 100) ** 2) * 10) / 10 : null);

  const history = $derived(
    [...weight.list]
      .sort((a, b) => b.date.localeCompare(a.date))
      .map((w, i, all) => ({ w, change: all[i + 1] ? w.weight_kg - all[i + 1].weight_kg : null })),
  );

  function rateText(r: number) {
    if (Math.abs(r) < 0.05) return 'Holding steady';
    return `${r < 0 ? 'Losing' : 'Gaining'} ${formatPace(Math.round(Math.abs(r) * 100) / 100, units)}`;
  }

  async function removed(w: Weighin) {
    editing = null;
    try {
      const putBack = await weight.remove(w);
      clearTimeout(undoTimer);
      undo = { label: `Removed the weigh-in of ${shortDate(w.date, today)}`, run: putBack };
      undoTimer = setTimeout(() => (undo = null), 6000);
    } catch (err) {
      weight.error = errorMessage(err, "Couldn't remove it. Please try again.");
    }
  }

  async function runUndo() {
    const u = undo;
    undo = null;
    try {
      await u?.run();
    } catch (err) {
      weight.error = errorMessage(err, "Couldn't put it back.");
    }
  }
</script>

<header class="flex items-center justify-between">
  <h1 class="text-2xl font-bold tracking-tight">Weight</h1>
  <button class="btn-primary px-4 py-2 text-sm" onclick={() => (logging = true)}>Log weight</button>
</header>

{#if weight.error}
  <p
    class="rounded-xl bg-red-50 px-4 py-2 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-300"
    role="alert"
  >
    {weight.error}
  </p>
{/if}

{#if weight.loaded && !now}
  <section class="card flex flex-col items-center gap-3 text-center">
    <img src="/brand/plato.svg" alt="" class="size-24" />
    <p class="font-medium">Log your weight to start your chart.</p>
    <button class="btn-primary" onclick={() => (logging = true)}>Log weight</button>
  </section>
{:else if now}
  {#if reached}
    <section
      class="card flex items-center gap-4 border-orange-200 bg-orange-50 dark:border-orange-900 dark:bg-orange-950/40"
    >
      <img src="/brand/plato-cheer.svg" alt="" class="size-16 shrink-0" />
      <div>
        <p class="font-semibold">
          You reached your goal of {formatWeight(plan.goal_weight_kg, units)}!
        </p>
        <p class="mt-0.5 text-sm text-zinc-600 dark:text-zinc-400">
          Your budget now keeps you here. Set a new goal whenever you like.
        </p>
        <button
          class="mt-2 text-sm font-semibold text-brand-600 dark:text-brand-400"
          onclick={() => profile.redo()}>Set a new goal</button
        >
      </div>
    </section>
  {/if}

  <section class="card">
    <div class="flex items-end justify-between gap-4">
      <div>
        <p class="text-sm text-zinc-500 dark:text-zinc-400">Now</p>
        <p class="text-4xl font-bold tracking-tight tabular-nums">{formatWeight(now.kg, units)}</p>
      </div>
      {#if start && start.date !== now.date}
        <div class="text-right">
          <p class="text-lg font-semibold tabular-nums">{formatChange(now.kg - start.kg, units)}</p>
          <p class="text-sm text-zinc-500 dark:text-zinc-400">
            since {shortDate(start.date, today)}
          </p>
        </div>
      {/if}
    </div>
    {#if hasGoal && start}
      <div
        class="mt-4 h-2 overflow-hidden rounded-full bg-orange-100 dark:bg-zinc-800"
        role="progressbar"
        aria-label="Progress to your goal"
        aria-valuenow={Math.round(done * 100)}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          class="h-full rounded-full bg-orange-500 transition-all"
          style="width: {done * 100}%"
        ></div>
      </div>
      <div class="mt-1.5 flex justify-between text-xs text-zinc-500 dark:text-zinc-400">
        <span>{formatWeight(start.kg, units)}</span>
        {#if !reached}
          <span>{formatWeight(Math.abs(now.kg - plan.goal_weight_kg), units)} to go</span>
        {/if}
        <span>Goal {formatWeight(plan.goal_weight_kg, units)}</span>
      </div>
    {/if}
  </section>

  <div class="flex gap-1" role="radiogroup" aria-label="Time range">
    {#each ranges as r (r.id)}
      <button
        class="chip flex-1 px-0 py-1.5"
        class:chosen={range === r.id}
        role="radio"
        aria-checked={range === r.id}
        onclick={() => (range = r.id)}>{r.label}</button
      >
    {/each}
  </div>

  <section class="card px-3 pb-3">
    {#if shown.length}
      <WeightChart points={shown} goal={hasGoal ? plan.goal_weight_kg : null} {units} {today} />
      <div class="mt-2 flex gap-4 px-2 text-xs text-zinc-500 dark:text-zinc-400">
        <span class="flex items-center gap-1.5"
          ><span class="inline-block size-2 rounded-full bg-zinc-300 dark:bg-zinc-600"
          ></span>Weigh-ins</span
        >
        <span class="flex items-center gap-1.5"
          ><span class="inline-block h-0.5 w-3 rounded bg-orange-500"></span>Trend</span
        >
        {#if hasGoal}
          <span class="flex items-center gap-1.5"
            ><span class="inline-block w-3 border-t border-dashed border-zinc-400"></span>Goal</span
          >
        {/if}
      </div>
    {:else}
      <p class="py-10 text-center text-sm text-zinc-500 dark:text-zinc-400">
        No weigh-ins in this time. Pick a longer range.
      </p>
    {/if}
  </section>

  <section class="card grid gap-3 text-sm">
    <div class="flex justify-between gap-4">
      <span class="text-zinc-500 dark:text-zinc-400">Last 4 weeks</span>
      <span class="text-right font-medium"
        >{rate === null ? 'Weigh in a few times a week to see your pace' : rateText(rate)}</span
      >
    </div>
    {#if hasGoal && !reached}
      {#if rate !== null}
        <div class="flex justify-between gap-4">
          <span class="text-zinc-500 dark:text-zinc-400">At this pace</span>
          <span class="text-right font-medium"
            >{eta ? `Goal around ${shortDate(eta, today)}` : 'Not heading to your goal yet'}</span
          >
        </div>
      {/if}
      {#if plan.goal_date}
        <div class="flex justify-between gap-4">
          <span class="text-zinc-500 dark:text-zinc-400">Your plan</span>
          <span class="text-right font-medium"
            >{formatPace(plan.pace_kg, units)}, goal by {shortDate(plan.goal_date, today)}</span
          >
        </div>
      {/if}
    {/if}
    {#if bmi}
      <div class="flex justify-between gap-4">
        <span class="text-zinc-500 dark:text-zinc-400">BMI {bmi}</span>
        <span class="text-right font-medium"
          >Healthy range {formatWeight(healthy[0], units)}–{formatWeight(healthy[1], units)}</span
        >
      </div>
    {/if}
  </section>

  <section class="card p-0">
    <h2 class="px-5 pt-4 pb-2 font-semibold">Weigh-ins</h2>
    <ul class="divide-y divide-zinc-100 dark:divide-zinc-800">
      {#each allShown ? history : history.slice(0, 10) as { w, change } (w.id)}
        <li>
          <button
            class="flex w-full items-center justify-between gap-3 px-5 py-2.5 text-left hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
            onclick={() => (editing = w)}
          >
            <span class="min-w-0">
              <span class="block">{shortDate(w.date, today)}</span>
              {#if w.note}<span class="block truncate text-sm text-zinc-500 dark:text-zinc-400"
                  >{w.note}</span
                >{/if}
            </span>
            <span class="shrink-0 text-right tabular-nums">
              <span class="block font-medium">{formatWeight(w.weight_kg, units)}</span>
              {#if change !== null}<span class="block text-xs text-zinc-500 dark:text-zinc-400"
                  >{formatChange(change, units)}</span
                >{/if}
            </span>
          </button>
        </li>
      {/each}
    </ul>
    {#if history.length > 10 && !allShown}
      <button
        class="w-full rounded-b-2xl border-t border-zinc-100 px-5 py-3 text-left text-sm font-semibold text-brand-600 hover:bg-zinc-50 dark:border-zinc-800 dark:text-brand-400 dark:hover:bg-zinc-800/50"
        onclick={() => (allShown = true)}>Show all {history.length}</button
      >
    {/if}
  </section>
{/if}

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

{#if logging}
  <LogWeight {units} onclose={() => (logging = false)} />
{/if}
{#if editing}
  <LogWeight {units} {editing} onclose={() => (editing = null)} onremove={removed} />
{/if}
