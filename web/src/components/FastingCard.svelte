<script lang="ts">
  import { fastingState, formatDuration } from '../lib/extras';
  import { fastings, type Fasting } from '../lib/profile';

  // Today's fasting window: how long until it opens, or until it closes.
  let { start, fasting }: { start: string; fasting: Fasting } = $props();

  let now = $state(new Date());
  $effect(() => {
    const t = setInterval(() => (now = new Date()), 30_000);
    return () => clearInterval(t);
  });

  const fast = $derived(fastingState(now, start, fasting));
  const time = (d: Date) => d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
</script>

{#if fast}
  <section class="card">
    <div class="flex items-baseline justify-between gap-3">
      <h2 class="font-semibold">{fast.eating ? 'Eating window' : 'Fasting'}</h2>
      <span class="text-sm text-zinc-500 dark:text-zinc-400"
        >{fastings.find((f) => f.id === fasting)?.label}</span
      >
    </div>
    <p class="mt-1 text-2xl font-bold tracking-tight tabular-nums">
      {formatDuration(fast.until.getTime() - now.getTime())}
    </p>
    <p class="text-sm text-zinc-500 dark:text-zinc-400">
      {fast.eating ? 'until it closes' : 'until you can eat'} at {time(fast.until)}
    </p>
    <div
      class="mt-3 h-1.5 overflow-hidden rounded-full bg-orange-100 dark:bg-zinc-800"
      role="progressbar"
      aria-label={fast.eating ? 'Eating window' : 'Fast'}
      aria-valuenow={Math.round(fast.progress * 100)}
      aria-valuemin="0"
      aria-valuemax="100"
    >
      <div
        class="h-full rounded-full {fast.eating ? 'bg-emerald-500' : 'bg-orange-500'}"
        style="width: {Math.min(fast.progress * 100, 100)}%"
      ></div>
    </div>
  </section>
{/if}
