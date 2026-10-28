<script lang="ts">
  import { stepServings, type Food } from '../lib/diary';
  import { countText, gramsOf, readAmount, unitsOf, type Unit } from '../lib/portions';
  import Stepper from './Stepper.svelte';

  // How much of a food: a count of a unit that suits it (2 tbsp of hummus,
  // 1 slice of bread), or grams. The amount is kept as text, such as "2 tbsp",
  // for the diary to show; it is empty when the amount is in grams.
  let {
    food,
    grams = $bindable(),
    amount = $bindable(''),
  }: { food: Food; grams: number | undefined; amount: string } = $props();

  const units = $derived(unitsOf(food));
  const chosen = $derived(readAmount(amount, units));

  function use(u: Unit, count = 1) {
    amount = countText(u, count);
    grams = gramsOf(u, count);
  }
</script>

{#if units.length}
  <div class="flex flex-wrap gap-2" role="radiogroup" aria-label="Unit">
    {#each units as u (u.one)}
      <button
        type="button"
        class="chip"
        class:chosen={chosen?.unit.one === u.one}
        role="radio"
        aria-checked={chosen?.unit.one === u.one}
        onclick={() => use(u)}>{u.one}</button
      >
    {/each}
    <button
      type="button"
      class="chip"
      class:chosen={!chosen}
      role="radio"
      aria-checked={!chosen}
      onclick={() => (amount = '')}>grams</button
    >
  </div>
{/if}

{#if chosen}
  <div class="card py-3">
    <Stepper
      label={countText(chosen.unit, chosen.count)}
      hint="{grams ?? 0} g"
      canDown={chosen.count > 0.5}
      ondown={() => use(chosen.unit, stepServings(chosen.count, false))}
      onup={() => use(chosen.unit, stepServings(chosen.count, true))}
    />
  </div>
{:else}
  <label>
    Amount (g)
    <input
      type="number"
      inputmode="decimal"
      min="1"
      max="5000"
      required
      bind:value={grams}
      oninput={() => (amount = '')}
    />
  </label>
  <div class="flex flex-wrap gap-2">
    {#each [50, 100, 150, 200] as g (g)}
      <button type="button" class="chip" class:chosen={grams === g} onclick={() => (grams = g)}
        >{g} g</button
      >
    {/each}
  </div>
{/if}
