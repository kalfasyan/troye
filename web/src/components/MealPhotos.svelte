<script lang="ts">
  import type { MealPhoto } from '../lib/photos';
  import { photos } from '../lib/photos.svelte';
  import LoggedPhoto from './LoggedPhoto.svelte';

  // The photos taken of a meal that is in the diary; one opens with the food
  // it was logged as.
  let { list, title }: { list: MealPhoto[]; title: string } = $props();

  let thumbs = $state<Record<string, string>>({});
  let open = $state<MealPhoto | null>(null);

  $effect(() => {
    for (const p of list) {
      if (!thumbs[p.id]) photos.url(p, true).then((u) => (thumbs[p.id] = u));
    }
  });
</script>

{#if list.length}
  <div class="flex gap-2 overflow-x-auto px-5 pb-2">
    {#each list as p (p.id)}
      <button
        class="size-16 shrink-0 overflow-hidden rounded-xl bg-zinc-100 dark:bg-zinc-800"
        aria-label="Photo of {title.toLowerCase()}"
        onclick={() => (open = p)}
      >
        {#if thumbs[p.id]}<img src={thumbs[p.id]} alt="" class="size-full object-cover" />{/if}
      </button>
    {/each}
  </div>
{/if}

{#if open}
  <LoggedPhoto photo={open} onclose={() => (open = null)} />
{/if}
