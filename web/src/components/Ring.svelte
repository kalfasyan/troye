<script lang="ts">
  // The day's calorie ring: it fills clockwise from 12 o'clock as food is
  // logged, and turns rose once the budget is passed (docs/brand.md).
  let { eaten, budget }: { eaten: number; budget: number } = $props();

  const r = 52;
  const length = 2 * Math.PI * r;
  const over = $derived(budget > 0 && eaten > budget);
  const fill = $derived(budget > 0 ? Math.min(eaten / budget, 1) : 0);
  const left = $derived(Math.round(budget - eaten));
</script>

<div class="relative mx-auto size-48">
  <svg viewBox="0 0 120 120" class="size-full -rotate-90" aria-hidden="true">
    <circle
      cx="60"
      cy="60"
      {r}
      fill="none"
      stroke-width="12"
      class="stroke-orange-200 dark:stroke-orange-950"
    />
    {#if fill > 0}
      <circle
        cx="60"
        cy="60"
        {r}
        fill="none"
        stroke-width="12"
        stroke-linecap="round"
        stroke-dasharray="{fill * length} {length}"
        class="transition-[stroke-dasharray] duration-500 {over
          ? 'stroke-rose-500'
          : 'stroke-orange-500'}"
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
