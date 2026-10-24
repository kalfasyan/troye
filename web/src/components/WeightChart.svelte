<script lang="ts">
  import { kgToLb, type Units } from '../lib/profile';
  import { daysBetween, shortDate, ticks, type Point } from '../lib/weight';

  // Weight over time: each weigh-in as a dot, the smoothed trend as the line,
  // and the goal as a dashed line when it is close enough to show. Touching or
  // hovering picks the nearest day, with its numbers beside it; the arrow
  // keys step through them.
  let {
    points,
    goal,
    units,
    today,
  }: { points: Point[]; goal: number | null; units: Units; today: string } = $props();

  let width = $state(320);
  const height = 200;
  const pad = { top: 14, right: 12, bottom: 24, left: 34 };

  const show = (kg: number) => (units === 'imperial' ? kgToLb(kg) : kg);
  const unit = $derived(units === 'imperial' ? 'lb' : 'kg');

  const first = $derived(points[0]?.date ?? today);
  const last = $derived(points.at(-1)?.date ?? today);
  const span = $derived(Math.max(daysBetween(first, last), 1));

  // the goal is drawn when it is within a few data spans, so it doesn't flatten the line
  const values = $derived(points.flatMap((p) => [show(p.kg), show(p.trend)]));
  const lo = $derived(Math.min(...values));
  const hi = $derived(Math.max(...values));
  const goalShown = $derived(
    goal !== null &&
      show(goal) >= lo - Math.max(hi - lo, 2) * 1.5 &&
      show(goal) <= hi + Math.max(hi - lo, 2) * 1.5,
  );
  const yTicks = $derived(
    ticks(Math.min(lo, goalShown ? show(goal!) : lo), Math.max(hi, goalShown ? show(goal!) : hi)),
  );
  const yMin = $derived(yTicks[0]);
  const yMax = $derived(yTicks.at(-1)!);

  const x = (date: string) =>
    points.length === 1
      ? pad.left + (width - pad.left - pad.right) / 2
      : pad.left + (daysBetween(first, date) / span) * (width - pad.left - pad.right);
  const y = (v: number) =>
    pad.top + (1 - (v - yMin) / (yMax - yMin || 1)) * (height - pad.top - pad.bottom);

  const trendPath = $derived(
    points
      .map((p, i) => `${i ? 'L' : 'M'}${x(p.date).toFixed(1)},${y(show(p.trend)).toFixed(1)}`)
      .join(''),
  );
  const xLabels = $derived(
    points.length > 1 ? [first, points[Math.floor(points.length / 2)].date, last] : [first],
  );

  let picked = $state<number | null>(null);
  const pick = $derived(picked !== null ? points[picked] : null);

  function nearest(e: PointerEvent) {
    const box = (e.currentTarget as SVGElement).getBoundingClientRect();
    const px = e.clientX - box.left;
    let best = 0;
    for (let i = 1; i < points.length; i++) {
      if (Math.abs(x(points[i].date) - px) < Math.abs(x(points[best].date) - px)) best = i;
    }
    picked = best;
  }

  function key(e: KeyboardEvent) {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
    e.preventDefault();
    const step = e.key === 'ArrowLeft' ? -1 : 1;
    picked = Math.min(Math.max((picked ?? points.length) + step, 0), points.length - 1);
  }

  const fmt = (v: number) => (Math.round(v * 10) / 10).toLocaleString();
</script>

<div class="relative" bind:clientWidth={width}>
  <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
  <svg
    {width}
    {height}
    class="block touch-pan-y outline-none select-none"
    role="img"
    aria-label="Weight chart. Use the arrow keys to read each weigh-in."
    tabindex="0"
    onpointermove={nearest}
    onpointerdown={nearest}
    onpointerleave={(e) => e.pointerType === 'mouse' && (picked = null)}
    onkeydown={key}
    onblur={() => (picked = null)}
  >
    {#each yTicks as t (t)}
      <line
        x1={pad.left}
        x2={width - pad.right}
        y1={y(t)}
        y2={y(t)}
        class="stroke-zinc-100 dark:stroke-zinc-800"
      />
      <text
        x={pad.left - 6}
        y={y(t)}
        dy="0.32em"
        text-anchor="end"
        class="fill-zinc-500 text-[11px] tabular-nums dark:fill-zinc-400">{t}</text
      >
    {/each}
    {#each xLabels as d, i (d + i)}
      <text
        x={x(d)}
        y={height - 6}
        text-anchor={xLabels.length === 1
          ? 'middle'
          : i === 0
            ? 'start'
            : i === 2
              ? 'end'
              : 'middle'}
        class="fill-zinc-500 text-[11px] dark:fill-zinc-400">{shortDate(d, today)}</text
      >
    {/each}

    {#if goalShown}
      <line
        x1={pad.left}
        x2={width - pad.right}
        y1={y(show(goal!))}
        y2={y(show(goal!))}
        stroke-dasharray="4 4"
        class="stroke-zinc-400 dark:stroke-zinc-500"
      />
      <text
        x={width - pad.right}
        y={y(show(goal!)) - 5}
        text-anchor="end"
        class="fill-zinc-500 text-[11px] font-medium dark:fill-zinc-400"
        >Goal {fmt(show(goal!))}</text
      >
    {/if}

    {#each points as p (p.date)}
      <circle
        cx={x(p.date)}
        cy={y(show(p.kg))}
        r="3.5"
        stroke-width="1.5"
        class="fill-zinc-300 stroke-white dark:fill-zinc-600 dark:stroke-zinc-900"
      />
    {/each}
    <path
      d={trendPath}
      fill="none"
      stroke-width="2"
      stroke-linejoin="round"
      stroke-linecap="round"
      class="stroke-orange-500"
    />

    {#if pick}
      <line
        x1={x(pick.date)}
        x2={x(pick.date)}
        y1={pad.top}
        y2={height - pad.bottom}
        class="stroke-zinc-300 dark:stroke-zinc-600"
      />
      <circle
        cx={x(pick.date)}
        cy={y(show(pick.kg))}
        r="5"
        stroke-width="2"
        class="fill-zinc-900 stroke-white dark:fill-zinc-100 dark:stroke-zinc-900"
      />
    {/if}
  </svg>

  {#if pick}
    <div
      class="pointer-events-none absolute top-0 rounded-lg border border-zinc-200 bg-white px-2.5 py-1.5 text-xs shadow-sm dark:border-zinc-700 dark:bg-zinc-800"
      style="left: {x(pick.date) > width / 2
        ? x(pick.date) - 132
        : x(pick.date) + 12}px; width: 120px"
      aria-live="polite"
    >
      <b class="block text-sm tabular-nums">{fmt(show(pick.kg))} {unit}</b>
      <span class="flex items-center gap-1.5 text-zinc-500 dark:text-zinc-400">
        <span class="inline-block h-0.5 w-3 rounded bg-orange-500"></span>
        trend {fmt(show(pick.trend))}
      </span>
      <span class="block text-zinc-500 dark:text-zinc-400">{shortDate(pick.date, today)}</span>
    </div>
  {/if}
</div>
