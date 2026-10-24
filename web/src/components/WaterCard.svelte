<script lang="ts">
  import { GLASS_ML, WATER_GOAL_ML } from '../lib/extras';
  import { extras } from '../lib/extras.svelte';
  import { errorMessage } from '../lib/pb';

  // The day's water in glasses: a tap on + adds one, − takes one back.
  let { onerror }: { onerror: (message: string) => void } = $props();

  const goal = WATER_GOAL_ML / GLASS_ML;
  const glasses = $derived(Math.round(extras.ml / GLASS_ML));

  async function change(by: number) {
    try {
      await extras.setWater(extras.ml + by * GLASS_ML);
    } catch (err) {
      onerror(errorMessage(err, "Couldn't save your water."));
    }
  }
</script>

<section class="card flex items-center gap-4">
  <div class="min-w-0 flex-1">
    <h2 class="font-semibold">Water</h2>
    <p class="text-sm text-zinc-500 tabular-nums dark:text-zinc-400">
      {(extras.ml / 1000).toLocaleString(undefined, { maximumFractionDigits: 2 })} of {WATER_GOAL_ML /
        1000} L
    </p>
    <div class="mt-2 flex gap-1" aria-hidden="true">
      {#each { length: Math.max(goal, glasses) } as _, i (i)}
        <span
          class="h-5 flex-1 rounded-b-md rounded-t-sm {i < glasses
            ? 'bg-sky-500'
            : 'bg-sky-100 dark:bg-sky-950'}"
        ></span>
      {/each}
    </div>
  </div>
  <div class="flex shrink-0 gap-2">
    <button
      class="btn-secondary size-11 rounded-full p-0 text-xl"
      aria-label="One glass less"
      disabled={extras.ml <= 0}
      onclick={() => change(-1)}>−</button
    >
    <button
      class="btn-primary size-11 rounded-full p-0 text-xl"
      aria-label="Add a glass of {GLASS_ML} ml"
      onclick={() => change(1)}>+</button
    >
  </div>
</section>
