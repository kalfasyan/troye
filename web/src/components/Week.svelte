<script lang="ts">
  import { addDays, isoDay } from '../lib/diary';
  import type { Week, WeekDay } from '../lib/extras';
  import { loadWeek } from '../lib/extras.svelte';
  import { errorMessage } from '../lib/pb';
  import { formatWeight, type Units } from '../lib/profile';
  import Sheet from './Sheet.svelte';

  // A week at a glance: calories each day against that day's budget (its
  // mark), the averages and the streak. The arrows move between weeks; a tap
  // on a day shows its numbers.
  let { units, onclose }: { units: Units; onclose: () => void } = $props();

  const today = isoDay(new Date());
  let day = $state(today);
  let week = $state<Week | null>(null);
  let error = $state('');
  let picked = $state<WeekDay | null>(null);

  $effect(() => {
    const d = day;
    error = '';
    picked = null;
    loadWeek(d)
      .then((w) => d === day && (week = w))
      .catch((err) => (error = errorMessage(err, "Couldn't load the week.")));
  });

  const H = 140;
  const top = $derived(
    week ? Math.max(...week.days.flatMap((d) => [d.kcal, d.budget]), 1) * 1.1 : 1,
  );
  const y = (kcal: number) => H - (kcal / top) * H;
  const weekday = (d: string) =>
    new Date(d + 'T12:00:00').toLocaleDateString(undefined, { weekday: 'narrow' });
  const longDay = (d: string) =>
    new Date(d + 'T12:00:00').toLocaleDateString(undefined, {
      weekday: 'long',
      day: 'numeric',
      month: 'short',
    });
  const range = $derived(
    week
      ? `${new Date(week.start + 'T12:00:00').toLocaleDateString(undefined, { day: 'numeric', month: 'short' })} – ${new Date(addDays(week.start, 6) + 'T12:00:00').toLocaleDateString(undefined, { day: 'numeric', month: 'short' })}`
      : '',
  );
  const change = $derived.by(() => {
    if (!week?.has_weight) return '–';
    const c = week.weight_change;
    if (c === 0) return 'Steady';
    return (c > 0 ? '+' : '−') + formatWeight(Math.abs(c), units);
  });
</script>

<Sheet title="Your week" {onclose}>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <button
        class="rounded-full p-2 text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800"
        aria-label="Previous week"
        onclick={() => (day = addDays(week?.start ?? day, -7))}
      >
        <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2"
          ><path d="M15 5l-7 7 7 7" stroke-linecap="round" stroke-linejoin="round" /></svg
        >
      </button>
      <span class="font-medium">{range}</span>
      <button
        class="rounded-full p-2 text-zinc-500 hover:bg-zinc-100 disabled:invisible dark:hover:bg-zinc-800"
        aria-label="Next week"
        disabled={!week || addDays(week.start, 7) > today}
        onclick={() => (day = addDays(week!.start, 7))}
      >
        <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2"
          ><path d="M9 5l7 7-7 7" stroke-linecap="round" stroke-linejoin="round" /></svg
        >
      </button>
    </div>

    {#if error}<p class="text-sm text-red-600 dark:text-red-400" role="alert">{error}</p>{/if}

    {#if week}
      <section class="card">
        <h3 class="text-sm text-zinc-500 dark:text-zinc-400">Calories against your budget</h3>
        <svg
          viewBox="0 0 280 {H + 22}"
          class="mt-3 w-full overflow-visible"
          role="img"
          aria-label="Calories each day of the week"
        >
          <line x1="0" x2="280" y1={H} y2={H} class="stroke-zinc-200 dark:stroke-zinc-800" />
          {#each week.days as d, i (d.date)}
            {@const x = i * 40 + 20}
            {@const over = d.kcal > d.budget}
            <g
              role="button"
              tabindex="0"
              aria-label="{longDay(d.date)}: {d.logged
                ? `${Math.round(d.kcal)} of ${d.budget} kcal`
                : 'nothing logged'}"
              class="cursor-pointer outline-none"
              onclick={() => (picked = picked?.date === d.date ? null : d)}
              onkeydown={(e) =>
                (e.key === 'Enter' || e.key === ' ') &&
                (picked = picked?.date === d.date ? null : d)}
            >
              <rect x={x - 20} y="0" width="40" height={H + 22} fill="transparent" />
              {#if picked?.date === d.date}
                <rect
                  x={x - 18}
                  y="0"
                  width="36"
                  height={H}
                  rx="6"
                  class="fill-zinc-100 dark:fill-zinc-800/60"
                />
              {/if}
              {#if d.logged && d.kcal > 0}
                <path
                  d="M{x - 10},{H} V{y(d.kcal) + 4} a4,4 0 0 1 4,-4 h12 a4,4 0 0 1 4,4 V{H} Z"
                  class={over ? 'fill-rose-500' : 'fill-orange-500'}
                />
              {/if}
              {#if d.date <= today}
                <line
                  x1={x - 15}
                  x2={x + 15}
                  y1={y(d.budget)}
                  y2={y(d.budget)}
                  stroke-width="2"
                  stroke-linecap="round"
                  class="stroke-zinc-400 dark:stroke-zinc-500"
                />
              {/if}
              <text
                {x}
                y={H + 16}
                text-anchor="middle"
                class="text-[11px] {d.date === today
                  ? 'fill-zinc-900 font-semibold dark:fill-zinc-100'
                  : 'fill-zinc-500 dark:fill-zinc-400'}">{weekday(d.date)}</text
              >
            </g>
          {/each}
        </svg>
        <p class="mt-2 min-h-10 text-sm text-zinc-600 dark:text-zinc-400">
          {#if picked}
            <b class="text-zinc-900 dark:text-zinc-100">{longDay(picked.date)}</b>:
            {#if picked.logged}
              {Math.round(picked.kcal).toLocaleString()} of {picked.budget.toLocaleString()} kcal{picked.exercise
                ? `, with ${picked.exercise} from exercise`
                : ''}{picked.water_ml ? `, ${picked.water_ml / 1000} L water` : ''}
            {:else}
              nothing logged
            {/if}
          {:else}
            The grey mark is each day's budget, exercise included. Tap a day for its numbers.
          {/if}
        </p>
      </section>

      <section class="grid grid-cols-2 gap-3">
        <div class="card">
          <p class="text-sm text-zinc-500 dark:text-zinc-400">Days logged</p>
          <p class="text-2xl font-bold tabular-nums">
            {week.days_logged} <span class="text-base font-normal text-zinc-500">of 7</span>
          </p>
        </div>
        <div class="card">
          <p class="text-sm text-zinc-500 dark:text-zinc-400">Within budget</p>
          <p class="text-2xl font-bold tabular-nums">
            {week.under_budget}
            <span class="text-base font-normal text-zinc-500"
              >{week.under_budget === 1 ? 'day' : 'days'}</span
            >
          </p>
        </div>
        <div class="card">
          <p class="text-sm text-zinc-500 dark:text-zinc-400">Average a day</p>
          <p class="text-2xl font-bold tabular-nums">
            {week.days_logged ? week.avg_kcal.toLocaleString() : '–'}
            <span class="text-base font-normal text-zinc-500">kcal</span>
          </p>
        </div>
        <div class="card">
          <p class="text-sm text-zinc-500 dark:text-zinc-400">Weight</p>
          <p class="text-2xl font-bold tabular-nums">{change}</p>
        </div>
      </section>

      <section class="card flex items-center gap-4">
        <span class="text-3xl" aria-hidden="true">🔥</span>
        <div>
          <p class="font-semibold">
            {week.streak === 1 ? '1 day' : `${week.streak} days`} in a row
          </p>
          <p class="text-sm text-zinc-500 dark:text-zinc-400">
            {week.streak
              ? 'Log something today to keep it going.'
              : 'Log a meal to start a streak.'}
          </p>
        </div>
      </section>
    {/if}
  </div>
</Sheet>
