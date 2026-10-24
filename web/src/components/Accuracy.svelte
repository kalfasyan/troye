<script lang="ts">
  import { onMount } from 'svelte';
  import { pct, scoreModels, type ModelScore } from '../lib/accuracy';
  import { pb } from '../lib/pb';
  import type { MealPhoto } from '../lib/photos';
  import { session } from '../lib/session.svelte';

  // How close the photo analyser's drafts came to what you logged, per model.
  // Nothing shows until a photo has been logged.
  let scores = $state<ModelScore[]>([]);

  onMount(async () => {
    try {
      const list = await pb.collection('meal_photos').getList<MealPhoto>(1, 500, {
        filter: pb.filter('user = {:user} && status = "logged"', { user: session.user!.id }),
        fields: 'model,items,confirmed',
        sort: '-created',
        requestKey: null,
      });
      scores = scoreModels(list.items);
    } catch {
      // a nice-to-have; leave it out
    }
  });

  const bias = (x: number | null) =>
    x === null || Math.abs(x) < 0.03 ? 'about right' : x > 0 ? 'too high' : 'too low';
</script>

{#if scores.length}
  <section class="card">
    <h2 class="font-semibold">Photo analyser</h2>
    <p class="text-sm text-zinc-500 dark:text-zinc-400">
      How its drafts compare with what you logged.
    </p>
    {#each scores as s (s.model)}
      <div class="mt-4">
        <p class="flex justify-between text-sm">
          <span class="font-medium">{s.model}</span>
          <span class="text-zinc-500 dark:text-zinc-400"
            >{s.photos} photo{s.photos === 1 ? '' : 's'}</span
          >
        </p>
        <dl class="mt-2 grid grid-cols-3 gap-2 text-center text-sm">
          <div class="rounded-xl bg-zinc-50 py-2 dark:bg-zinc-800/60">
            <dt class="text-xs text-zinc-500 dark:text-zinc-400">Calories off</dt>
            <dd class="text-lg font-semibold tabular-nums">{pct(s.kcalOff)}</dd>
            <dd class="text-xs text-zinc-500 dark:text-zinc-400">{bias(s.kcalBias)}</dd>
          </div>
          <div class="rounded-xl bg-zinc-50 py-2 dark:bg-zinc-800/60">
            <dt class="text-xs text-zinc-500 dark:text-zinc-400">Foods found</dt>
            <dd class="text-lg font-semibold tabular-nums">{pct(s.found)}</dd>
            <dd class="text-xs text-zinc-500 dark:text-zinc-400">{pct(s.extra)} extra</dd>
          </div>
          <div class="rounded-xl bg-zinc-50 py-2 dark:bg-zinc-800/60">
            <dt class="text-xs text-zinc-500 dark:text-zinc-400">Portions off</dt>
            <dd class="text-lg font-semibold tabular-nums">{pct(s.gramsOff)}</dd>
            <dd class="text-xs text-zinc-500 dark:text-zinc-400">typically</dd>
          </div>
        </dl>
      </div>
    {/each}
  </section>
{/if}
