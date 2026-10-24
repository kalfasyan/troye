<script lang="ts">
  import { amountOf, meals, portions, type Entry, type Meal } from '../lib/diary';
  import { diary } from '../lib/diary.svelte';
  import { errorMessage } from '../lib/pb';
  import Sheet from './Sheet.svelte';

  // Changing or removing something already in the diary.
  let {
    entry,
    onclose,
    onremove,
  }: { entry: Entry; onclose: () => void; onremove: (e: Entry) => void } = $props();

  // svelte-ignore state_referenced_locally
  const food = entry.expand?.food;
  // svelte-ignore state_referenced_locally
  let meal = $state<Meal>(entry.meal);
  // svelte-ignore state_referenced_locally
  let grams = $state<number | undefined>(entry.grams || undefined);
  // svelte-ignore state_referenced_locally
  let kcal = $state<number | undefined>(entry.kcal);
  let busy = $state(false);
  let error = $state('');

  const preview = $derived(food && grams ? amountOf(food, grams) : null);

  async function save(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = '';
    try {
      await diary.update(entry, food ? { meal, grams } : { meal, kcal });
      onclose();
    } catch (err) {
      error = errorMessage(err, "Couldn't save. Please try again.");
    } finally {
      busy = false;
    }
  }
</script>

<Sheet title={entry.name} {onclose}>
  <form class="flex flex-1 flex-col gap-4" onsubmit={save}>
    <div class="flex gap-1 overflow-x-auto" role="radiogroup" aria-label="Meal">
      {#each meals as m (m.id)}
        <button
          type="button"
          class="chip shrink-0 py-1.5"
          class:chosen={meal === m.id}
          role="radio"
          aria-checked={meal === m.id}
          onclick={() => (meal = m.id)}>{m.label}</button
        >
      {/each}
    </div>

    {#if food}
      <label>
        Amount (g)
        <input type="number" inputmode="decimal" min="1" max="5000" required bind:value={grams} />
      </label>
      <div class="flex flex-wrap gap-2">
        {#each portions(food) as p (p.label)}
          <button
            type="button"
            class="chip"
            class:chosen={grams === p.grams}
            onclick={() => (grams = p.grams)}>{p.label}</button
          >
        {/each}
      </div>
      {#if preview}
        <p class="text-sm text-zinc-600 dark:text-zinc-400">
          {preview.kcal} kcal · {preview.carbs} g carbs · {preview.protein} g protein · {preview.fat}
          g fat
        </p>
      {/if}
    {:else}
      <label>
        Calories
        <input type="number" inputmode="numeric" min="1" max="20000" required bind:value={kcal} />
      </label>
    {/if}

    {#if error}<p class="text-sm text-red-600 dark:text-red-400" role="alert">{error}</p>{/if}

    <div class="mt-auto grid grid-cols-2 gap-2">
      <button
        type="button"
        class="btn-secondary text-red-600 dark:text-red-400"
        onclick={() => onremove(entry)}>Remove</button
      >
      <button class="btn-primary" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
    </div>
  </form>
</Sheet>
