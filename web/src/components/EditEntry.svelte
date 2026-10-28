<script lang="ts">
  import {
    addOnIdeas,
    amountOf,
    entryAmount,
    meals,
    moreNutrients,
    scaleEntry,
    stepServings,
    type Entry,
    type Meal,
  } from '../lib/diary';
  import { diary } from '../lib/diary.svelte';
  import { errorMessage } from '../lib/pb';
  import { guessAmount } from '../lib/portions';
  import AmountPicker from './AmountPicker.svelte';
  import Sheet from './Sheet.svelte';
  import Stepper from './Stepper.svelte';

  // Changing or removing something already in the diary: how much, which meal,
  // and what was added to it (milk in a coffee, an egg on a soup).
  let {
    entry,
    onclose,
    onremove,
    onedit,
    onaddon,
  }: {
    entry: Entry;
    onclose: () => void;
    onremove: (e: Entry) => void;
    /** opens another entry, such as one of the add-ons */
    onedit?: (e: Entry) => void;
    /** adds something to this entry, searching for the words given; without it there are no add-ons here */
    onaddon?: (e: Entry, query: string) => void;
  } = $props();

  // svelte-ignore state_referenced_locally
  const food = entry.expand?.food;
  // svelte-ignore state_referenced_locally
  let meal = $state<Meal>(entry.meal);
  // svelte-ignore state_referenced_locally
  let grams = $state<number | undefined>(entry.grams || undefined);
  // svelte-ignore state_referenced_locally
  let amount = $state(entry.amount || (food ? guessAmount(food, entry.grams) : ''));
  // an entry without a food is scaled as a whole: 2 × "1 bowl" doubles everything
  let times = $state(1);
  // svelte-ignore state_referenced_locally
  let kcal = $state<number | undefined>(entry.kcal);
  let busy = $state(false);
  let error = $state('');

  const isAddOn = $derived(!!entry.parent);
  const addOns = $derived(diary.addOnsOf(entry));
  const parent = $derived(
    entry.parent ? diary.entries.find((e) => e.id === entry.parent) : undefined,
  );
  const preview = $derived(food && grams ? amountOf(food, grams) : null);
  const scaled = $derived(scaleEntry(entry, times));
  // what usually goes with it, less what is already in it
  const ideas = $derived(
    addOnIdeas(entry.name)
      .filter((w) => !addOns.some((a) => a.name.toLowerCase().includes(w)))
      .slice(0, 4),
  );

  function step(up: boolean) {
    times = stepServings(times, up);
    kcal = scaled.kcal;
  }

  async function save(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = '';
    try {
      const where = isAddOn ? {} : { meal };
      await diary.update(
        entry,
        food
          ? { ...where, grams, amount }
          : times !== 1
            ? { ...where, ...scaled, kcal }
            : { ...where, kcal },
      );
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
    {#if isAddOn}
      <p class="text-sm text-zinc-500 dark:text-zinc-400">
        Added to {#if parent && onedit}<button
            type="button"
            class="font-medium text-brand-600 underline-offset-2 hover:underline dark:text-brand-400"
            onclick={() => onedit(parent)}>{parent.name}</button
          >{:else if parent}{parent.name}{:else}another entry{/if}
      </p>
    {:else}
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
    {/if}

    {#if food}
      <AmountPicker {food} bind:grams bind:amount />
      {#if preview}
        <p class="text-sm text-zinc-600 dark:text-zinc-400">
          {preview.kcal} kcal · {preview.carbs} g carbs · {preview.protein} g protein · {preview.fat}
          g fat
        </p>
      {/if}
    {:else}
      <div class="card py-3">
        <Stepper
          label="{times} ×"
          hint={entryAmount({ ...entry, ...scaled }) || 'as logged'}
          canDown={times > 0.5}
          ondown={() => step(false)}
          onup={() => step(true)}
        />
      </div>
      <label>
        Calories
        <input type="number" inputmode="numeric" min="1" max="20000" required bind:value={kcal} />
      </label>
      {#if entry.amount || entry.protein || entry.carbs || entry.fat}
        <p class="text-sm text-zinc-600 dark:text-zinc-400">
          {[
            entryAmount({ ...entry, ...scaled }),
            `${scaled.carbs} g carbs · ${scaled.protein} g protein · ${scaled.fat} g fat`,
          ]
            .filter(Boolean)
            .join(' · ')}
        </p>
      {/if}
      {#if moreNutrients(entry).length}
        <p class="-mt-2 text-sm text-zinc-500 dark:text-zinc-400">
          {moreNutrients(entry).join(' · ')}
        </p>
      {/if}
    {/if}

    {#if !isAddOn && onaddon}
      <section>
        <h3 class="text-sm font-medium">
          {addOns.length ? 'Added to it' : 'Add something to it?'}
        </h3>
        {#if addOns.length}
          <ul
            class="mt-2 divide-y divide-zinc-100 rounded-xl border border-zinc-200 dark:divide-zinc-800 dark:border-zinc-800"
          >
            {#each addOns as a (a.id)}
              <li>
                <button
                  type="button"
                  class="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
                  onclick={() => onedit?.(a)}
                >
                  <span class="min-w-0 truncate">
                    {a.name}
                    {#if entryAmount(a)}<span class="text-sm text-zinc-500 dark:text-zinc-400">
                        · {entryAmount(a)}</span
                      >{/if}
                  </span>
                  <span class="shrink-0 text-sm tabular-nums">{Math.round(a.kcal)} kcal</span>
                </button>
              </li>
            {/each}
          </ul>
        {/if}
        <div class="mt-2 flex flex-wrap gap-2">
          {#each ideas as idea (idea)}
            <button type="button" class="chip" onclick={() => onaddon(entry, idea)}>+ {idea}</button
            >
          {/each}
          <button
            type="button"
            class="chip font-medium text-brand-600 dark:text-brand-400"
            onclick={() => onaddon(entry, '')}>+ Something else</button
          >
        </div>
      </section>
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
