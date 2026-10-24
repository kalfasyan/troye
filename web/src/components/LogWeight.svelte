<script lang="ts">
  import { isoDay } from '../lib/diary';
  import { errorMessage } from '../lib/pb';
  import { kgToLb, lbToKg, type Units } from '../lib/profile';
  import { weight } from '../lib/weight.svelte';
  import type { Weighin } from '../lib/weight';
  import Sheet from './Sheet.svelte';

  // Logging a weight, or changing or removing one already logged. It starts
  // from the last weight, so a small change is a tap or two on the steppers.
  let {
    units,
    editing = null,
    onclose,
    onremove,
  }: {
    units: Units;
    editing?: Weighin | null;
    onclose: () => void;
    onremove?: (w: Weighin) => void;
  } = $props();

  const today = isoDay(new Date());
  const show = (kg: number) => Math.round((units === 'imperial' ? kgToLb(kg) : kg) * 10) / 10;

  // svelte-ignore state_referenced_locally
  const start = editing?.weight_kg ?? weight.latest?.kg;
  // svelte-ignore state_referenced_locally
  let value = $state<number | undefined>(start ? show(start) : undefined);
  // svelte-ignore state_referenced_locally
  let date = $state(editing?.date ?? today);
  // svelte-ignore state_referenced_locally
  let note = $state(editing?.note ?? '');
  let busy = $state(false);
  let error = $state('');

  const unit = $derived(units === 'imperial' ? 'lb' : 'kg');
  const [min, max] = $derived(units === 'imperial' ? [66, 772] : [30, 350]);
  const replaces = $derived(!editing && weight.list.some((w) => w.date === date));

  const step = (d: number) => (value = Math.round(((value ?? 0) + d) * 10) / 10);

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!value) return;
    busy = true;
    error = '';
    try {
      const kg = units === 'imperial' ? Math.round(lbToKg(value) * 100) / 100 : value;
      if (editing && editing.date !== date) {
        // moving it to another day: replace the old one
        await weight.log(date, kg, note.trim());
        await weight.remove(editing);
      } else {
        await weight.log(date, kg, note.trim());
      }
      onclose();
    } catch (err) {
      error = errorMessage(err, "Couldn't save your weight. Please try again.");
    } finally {
      busy = false;
    }
  }
</script>

<Sheet title={editing ? 'Weigh-in' : 'Log weight'} {onclose}>
  <form class="flex flex-1 flex-col gap-4" onsubmit={save}>
    <div class="flex items-center justify-center gap-3 py-4">
      <button
        type="button"
        class="btn-secondary size-12 rounded-full p-0 text-xl"
        aria-label="Less"
        onclick={() => step(-0.1)}>−</button
      >
      <label class="shrink-0 flex-row items-baseline gap-1">
        <span class="sr-only">Weight in {unit}</span>
        <input
          class="w-36 border-0 px-0 bg-transparent text-center text-5xl font-bold tracking-tight tabular-nums shadow-none focus:ring-0 dark:bg-transparent"
          type="number"
          inputmode="decimal"
          step="0.1"
          {min}
          {max}
          required
          bind:value
        />
        <span class="text-lg font-normal text-zinc-500 dark:text-zinc-400">{unit}</span>
      </label>
      <button
        type="button"
        class="btn-secondary size-12 rounded-full p-0 text-xl"
        aria-label="More"
        onclick={() => step(0.1)}>+</button
      >
    </div>

    <label>Day <input type="date" max={today} required bind:value={date} /></label>
    {#if replaces}
      <p class="-mt-2 text-sm text-zinc-500 dark:text-zinc-400">
        This replaces the weight you logged that day.
      </p>
    {/if}
    <label
      >Note (optional)
      <input maxlength="200" placeholder="After a run, new scale…" bind:value={note} /></label
    >
    <p class="text-sm text-zinc-500 dark:text-zinc-400">
      Weigh yourself at the same time each day, ideally in the morning before breakfast. The trend
      line smooths out the daily ups and downs.
    </p>

    {#if error}<p class="text-sm text-red-600 dark:text-red-400" role="alert">{error}</p>{/if}

    <div class="mt-auto grid gap-2 {editing ? 'grid-cols-2' : ''}">
      {#if editing && onremove}
        <button
          type="button"
          class="btn-secondary text-red-600 dark:text-red-400"
          onclick={() => onremove(editing)}>Remove</button
        >
      {/if}
      <button class="btn-primary" disabled={busy || !value}>{busy ? 'Saving…' : 'Save'}</button>
    </div>
  </form>
</Sheet>
