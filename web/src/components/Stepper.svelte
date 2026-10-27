<script lang="ts">
  // A count with − and + buttons either side, like "2 × 1 egg".
  let {
    label,
    hint = '',
    ondown,
    onup,
    canDown = true,
  }: {
    label: string;
    hint?: string;
    ondown: () => void;
    onup: () => void;
    canDown?: boolean;
  } = $props();
</script>

{#snippet button(text: string, sign: string, onclick: () => void, disabled: boolean)}
  <button
    type="button"
    class="flex size-11 shrink-0 items-center justify-center rounded-full border border-zinc-300 bg-white text-xl font-semibold text-brand-600 transition hover:bg-zinc-100 active:scale-95 disabled:opacity-40 dark:border-zinc-700 dark:bg-zinc-900 dark:text-brand-400 dark:hover:bg-zinc-800"
    aria-label={text}
    {disabled}
    {onclick}>{sign}</button
  >
{/snippet}

<div class="flex items-center gap-3">
  {@render button('Less', '−', ondown, !canDown)}
  <div class="min-w-0 flex-1 text-center" aria-live="polite">
    <span class="block truncate text-lg font-semibold tabular-nums">{label}</span>
    {#if hint}<span class="block truncate text-sm text-zinc-500 dark:text-zinc-400">{hint}</span
      >{/if}
  </div>
  {@render button('More', '+', onup, false)}
</div>
