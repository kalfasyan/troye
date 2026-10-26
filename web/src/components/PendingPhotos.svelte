<script lang="ts">
  import { statusText, type MealPhoto } from '../lib/photos';
  import { photos } from '../lib/photos.svelte';

  // A meal's photos that aren't in the diary yet, under the meal they were
  // taken for: waiting for the PC, being analysed, ready to check or failed.
  let { list, onopen }: { list: MealPhoto[]; onopen: (p: MealPhoto) => void } = $props();

  let thumbs = $state<Record<string, string>>({});

  $effect(() => {
    for (const p of list) {
      if (!thumbs[p.id]) photos.url(p, true).then((u) => (thumbs[p.id] = u));
    }
  });
</script>

{#if list.length}
  <ul
    class="divide-y divide-zinc-100 border-t border-zinc-100 dark:divide-zinc-800 dark:border-zinc-800"
  >
    {#each list as p (p.id)}
      <li>
        <button
          class="flex w-full items-center gap-3 px-5 py-2.5 text-left hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
          onclick={() => onopen(p)}
        >
          {#if thumbs[p.id]}
            <img src={thumbs[p.id]} alt="" class="size-11 shrink-0 rounded-lg object-cover" />
          {:else}
            <span class="size-11 shrink-0 rounded-lg bg-zinc-100 dark:bg-zinc-800"></span>
          {/if}
          <span class="min-w-0 flex-1">
            <span class="block">Meal photo</span>
            <span
              class="block truncate text-sm {p.status === 'ready'
                ? 'font-medium text-brand-600 dark:text-brand-400'
                : p.status === 'failed'
                  ? 'text-rose-600 dark:text-rose-400'
                  : 'text-zinc-500 dark:text-zinc-400'}"
              >{statusText(p, photos.analyser.online)}</span
            >
          </span>
          {#if p.status === 'analysing'}
            <span
              class="size-4 shrink-0 animate-spin rounded-full border-2 border-orange-200 border-t-orange-500"
              aria-hidden="true"
            ></span>
          {:else if p.status === 'ready'}
            <span class="shrink-0 text-sm font-semibold text-brand-600 dark:text-brand-400"
              >Check</span
            >
          {/if}
        </button>
      </li>
    {/each}
  </ul>
{/if}
