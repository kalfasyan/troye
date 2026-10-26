<script lang="ts">
  import { diary } from '../lib/diary.svelte';
  import { summary, type ImportResult } from '../lib/loseit';
  import { errorMessage, pb } from '../lib/pb';
  import { profile } from '../lib/profile.svelte';
  import { weight } from '../lib/weight.svelte';
  import Sheet from './Sheet.svelte';

  // Bringing over everything from Lose It!: the food log, weigh-ins, exercise,
  // own foods, meal photos and, if wanted, the plan. Importing a newer export
  // later only adds what's new.
  let { onclose }: { onclose: () => void } = $props();

  let plan = $state(false);
  let busy = $state(false);
  let error = $state('');
  let result = $state<ImportResult | null>(null);
  let fileInput = $state<HTMLInputElement>();

  async function upload(e: Event) {
    const file = (e.currentTarget as HTMLInputElement).files?.[0];
    if (!file) return;
    busy = true;
    error = '';
    result = null;
    try {
      const body = new FormData();
      body.append('file', file);
      if (plan) body.append('plan', 'true');
      result = await pb.send<ImportResult>('/api/app/import/loseit', {
        method: 'POST',
        body,
        requestKey: null,
      });
      await Promise.all([profile.load(), diary.load(), weight.load()]);
    } catch (err) {
      error = errorMessage(err, "Couldn't import that file. Please try again.");
    } finally {
      busy = false;
      if (fileInput) fileInput.value = '';
    }
  }
</script>

<Sheet title="Import from Lose It!" {onclose}>
  <div class="flex flex-col gap-4">
    <p class="text-sm text-zinc-600 dark:text-zinc-400">
      Ask Lose It! for an export of your data in its settings. It arrives by email as a zip file:
      choose that file here. Your food log, weigh-ins, exercise, own foods and recipes, and meal
      photos come over, and stay on your own server.
    </p>

    <label class="flex-row items-start gap-3 font-normal">
      <input type="checkbox" class="mt-0.5 shrink-0" bind:checked={plan} disabled={busy} />
      <span>
        Use my Lose It! plan too: goal weight, pace, activity, and fasting schedule. Your daily
        budget is worked out again from it.
      </span>
    </label>

    <label class="btn-primary block cursor-pointer text-center" class:opacity-50={busy}>
      {busy ? 'Importing…' : 'Choose the export'}
      <input
        bind:this={fileInput}
        type="file"
        accept=".zip,application/zip"
        class="sr-only"
        disabled={busy}
        onchange={upload}
      />
    </label>
    {#if busy}
      <p class="text-sm text-zinc-500 dark:text-zinc-400">
        A big export with photos can take a minute to upload.
      </p>
    {/if}

    {#if result}
      <section class="card flex flex-col gap-2 text-sm" role="status">
        {#each summary(result) as line (line)}<p>{line}</p>{/each}
      </section>
      <p class="text-xs text-zinc-500 dark:text-zinc-400">
        Importing a newer export later only adds what's new.
      </p>
    {/if}

    {#if error}
      <p class="text-sm text-red-600 dark:text-red-400" role="alert">{error}</p>
    {/if}
  </div>
</Sheet>
