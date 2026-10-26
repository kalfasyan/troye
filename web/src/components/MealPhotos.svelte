<script lang="ts">
  import { errorMessage } from '../lib/pb';
  import type { MealPhoto } from '../lib/photos';
  import { photos } from '../lib/photos.svelte';
  import Sheet from './Sheet.svelte';

  // The photos taken of a meal that is in the diary; one opens full size.
  let { list, title, onerror }: { list: MealPhoto[]; title: string; onerror: (m: string) => void } =
    $props();

  let thumbs = $state<Record<string, string>>({});
  let open = $state<{ photo: MealPhoto; url: string } | null>(null);

  $effect(() => {
    for (const p of list) {
      if (!thumbs[p.id]) photos.url(p, true).then((u) => (thumbs[p.id] = u));
    }
  });

  async function show(p: MealPhoto) {
    try {
      open = { photo: p, url: await photos.url(p) };
    } catch (err) {
      onerror(errorMessage(err, "Couldn't open the photo."));
    }
  }

  async function remove(p: MealPhoto) {
    open = null;
    try {
      await photos.remove(p);
    } catch (err) {
      onerror(errorMessage(err, "Couldn't remove the photo."));
    }
  }
</script>

{#if list.length}
  <div class="flex gap-2 overflow-x-auto px-5 pb-2">
    {#each list as p (p.id)}
      <button
        class="size-16 shrink-0 overflow-hidden rounded-xl bg-zinc-100 dark:bg-zinc-800"
        aria-label="Photo of {title.toLowerCase()}"
        onclick={() => show(p)}
      >
        {#if thumbs[p.id]}<img src={thumbs[p.id]} alt="" class="size-full object-cover" />{/if}
      </button>
    {/each}
  </div>
{/if}

{#if open}
  <Sheet {title} onclose={() => (open = null)}>
    <img src={open.url} alt="Your {title.toLowerCase()}" class="w-full rounded-2xl" />
    <button
      class="btn-secondary mt-4 text-red-600 dark:text-red-400"
      onclick={() => open && remove(open.photo)}>Remove photo</button
    >
  </Sheet>
{/if}
