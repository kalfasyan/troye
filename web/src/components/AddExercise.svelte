<script lang="ts">
  import { exerciseKcal, exerciseKinds, type Exercise, type ExerciseKind } from '../lib/extras';
  import { extras } from '../lib/extras.svelte';
  import { errorMessage } from '../lib/pb';
  import { profile } from '../lib/profile.svelte';
  import Sheet from './Sheet.svelte';

  // Adding an exercise, or changing or removing one. The calories are an
  // estimate from the kind, minutes and weight; a watch's number can replace it.
  // An activity tracker's calories (imported from Lose It!) are only a number.
  let {
    editing = null,
    onclose,
    onremove,
  }: {
    editing?: Exercise | null;
    onclose: () => void;
    onremove?: (x: Exercise) => void;
  } = $props();

  // svelte-ignore state_referenced_locally
  let kind = $state<ExerciseKind>(editing?.kind ?? 'dog_walk');
  // svelte-ignore state_referenced_locally
  let minutes = $state<number | undefined>(editing?.minutes ?? 30);
  // svelte-ignore state_referenced_locally
  let name = $state(editing?.name ?? '');
  // the person's own calories; empty uses the estimate
  // svelte-ignore state_referenced_locally
  let own = $state<number | undefined>(
    editing &&
      editing.kcal !== exerciseKcal(editing.kind, editing.minutes, profile.current!.weight_kg)
      ? editing.kcal
      : undefined,
  );
  let busy = $state(false);
  let error = $state('');
  // svelte-ignore state_referenced_locally
  const tracker = editing?.kind === 'tracker';

  const estimate = $derived(exerciseKcal(kind, minutes ?? 0, profile.current!.weight_kg));

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (tracker ? !own : !minutes) return;
    busy = true;
    error = '';
    try {
      const kcal = own ?? estimate;
      if (editing)
        await extras.updateExercise(editing, {
          kind,
          minutes: minutes ?? 0,
          name: name.trim(),
          kcal,
        });
      else await extras.addExercise(kind, minutes ?? 0, name.trim(), kcal);
      onclose();
    } catch (err) {
      error = errorMessage(err, "Couldn't save it. Please try again.");
    } finally {
      busy = false;
    }
  }
</script>

<Sheet title={editing ? 'Exercise' : 'Add exercise'} {onclose}>
  <form class="flex flex-1 flex-col gap-4" onsubmit={save}>
    {#if tracker}
      <p class="text-sm text-zinc-600 dark:text-zinc-400">
        Extra calories your activity tracker counted, from {name || 'Lose It!'}.
      </p>
    {:else}
      <fieldset class="grid grid-cols-2 gap-2">
        <legend class="sr-only">Kind</legend>
        {#each exerciseKinds as k (k.id)}
          <button
            type="button"
            class="rounded-xl border px-3 py-2.5 text-left text-sm font-medium {kind === k.id
              ? 'border-brand-500 bg-brand-50 text-brand-700 dark:bg-brand-500/10 dark:text-brand-300'
              : 'border-zinc-200 dark:border-zinc-800'}"
            aria-pressed={kind === k.id}
            onclick={() => (kind = k.id)}>{k.label}</button
          >
        {/each}
      </fieldset>

      <label
        >Minutes
        <input type="number" inputmode="numeric" min="1" max="1440" required bind:value={minutes} />
      </label>
      <div class="flex flex-wrap gap-2">
        {#each [15, 30, 45, 60, 90] as m (m)}
          <button
            type="button"
            class="btn-secondary px-3 py-1.5 text-sm {minutes === m ? 'ring-2 ring-brand-500' : ''}"
            onclick={() => (minutes = m)}>{m} min</button
          >
        {/each}
      </div>
    {/if}

    <label
      >Calories burned
      <input
        type="number"
        inputmode="numeric"
        min="0"
        max="10000"
        placeholder={tracker ? '' : `${estimate} (estimate)`}
        required={tracker}
        bind:value={own}
      />
    </label>
    {#if !tracker}
      <p class="-mt-2 text-sm text-zinc-500 dark:text-zinc-400">
        What it burns on top of resting, added to the day's budget. Type your watch's number to use
        it instead.
      </p>
      <label
        >Note (optional)
        <input maxlength="100" placeholder="Park loop, spin class…" bind:value={name} /></label
      >
    {/if}

    {#if error}<p class="text-sm text-red-600 dark:text-red-400" role="alert">{error}</p>{/if}

    <div class="mt-auto grid gap-2 {editing ? 'grid-cols-2' : ''}">
      {#if editing && onremove}
        <button
          type="button"
          class="btn-secondary text-red-600 dark:text-red-400"
          onclick={() => onremove(editing)}>Remove</button
        >
      {/if}
      <button class="btn-primary" disabled={busy || (tracker ? !own : !minutes)}
        >{busy
          ? 'Saving…'
          : editing
            ? 'Save'
            : `Add ${(own ?? estimate).toLocaleString()} kcal`}</button
      >
    </div>
  </form>
</Sheet>
