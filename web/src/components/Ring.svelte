<script lang="ts">
  import { calorieSources, macroColors, type Totals } from '../lib/diary';

  // The day's calorie ring: it fills clockwise from 12 o'clock as food is
  // logged, in the colors of where the calories came from (carbs, protein,
  // fat, and grey for calories without macros). Past the budget, a rose arc
  // laps the ring from 12 o'clock again (docs/brand.md).
  let { totals, budget }: { totals: Totals; budget: number } = $props();

  const r = 52;
  const length = 2 * Math.PI * r;
  // a hairline between the colors, so they read as separate parts
  const gap = 0.8;
  const eaten = $derived(totals.kcal);
  const over = $derived(budget > 0 && eaten > budget);
  const fill = $derived(budget > 0 ? Math.min(eaten / budget, 1) : 0);
  const lap = $derived(over ? Math.min((eaten - budget) / budget, 1) : 0);
  const left = $derived(Math.round(budget - eaten));

  const segments = $derived.by(() => {
    const sources = calorieSources(totals);
    const parts = (['carbs', 'protein', 'fat', 'other'] as const)
      .map((key) => ({ key, size: eaten ? (sources[key] / eaten) * fill * length : 0 }))
      .filter((p) => p.size > 0.5);
    let at = 0;
    return parts.map((p, i) => {
      const start = at;
      at += p.size;
      // the last part keeps its full length so a full ring closes
      const dash = parts.length > 1 && (i < parts.length - 1 || fill < 1) ? p.size - gap : p.size;
      return { ...p, start, dash: Math.max(dash, 0.5) };
    });
  });
</script>

<div class="relative mx-auto size-48">
  <svg viewBox="0 0 120 120" class="size-full -rotate-90" aria-hidden="true">
    <circle
      cx="60"
      cy="60"
      {r}
      fill="none"
      stroke-width="12"
      class="stroke-orange-100 dark:stroke-zinc-800"
    />
    {#each segments as s (s.key)}
      <circle
        cx="60"
        cy="60"
        {r}
        fill="none"
        stroke-width="12"
        stroke-dasharray="{s.dash} {length}"
        stroke-dashoffset={-s.start}
        class="transition-all duration-500 {macroColors[s.key].stroke}"
      />
    {/each}
    {#if lap > 0}
      <circle
        cx="60"
        cy="60"
        {r}
        fill="none"
        stroke-width="12"
        stroke-linecap="round"
        stroke-dasharray="{lap * length} {length}"
        class="stroke-rose-500 transition-[stroke-dasharray] duration-500"
      />
    {/if}
  </svg>
  <div class="absolute inset-0 flex flex-col items-center justify-center text-center">
    <span class="text-4xl font-bold tracking-tight tabular-nums">
      {Math.abs(left).toLocaleString()}
    </span>
    <span
      class="text-sm {over
        ? 'font-medium text-rose-600 dark:text-rose-400'
        : 'text-zinc-500 dark:text-zinc-400'}"
    >
      {over ? 'over budget' : 'left'}
    </span>
  </div>
</div>
<p class="sr-only">
  {Math.round(eaten)} of {budget} calories eaten, {over ? `${-left} over` : `${left} left`}.
</p>
