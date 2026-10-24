<!--
  DEMO: a live list that people and workers add to. New items arrive over
  PocketBase's realtime connection, and everyone else gets a push notification.
  scripts/strip-demo.sh removes it.
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import type { RecordModel } from 'pocketbase';
  import { errorMessage, pb } from '../lib/pb';

  interface Item extends RecordModel {
    title: string;
    by: string;
    created: string;
  }

  let items = $state<Item[]>([]);
  let title = $state('');
  let error = $state('');

  const collection = pb.collection<Item>('items');

  onMount(() => {
    collection
      .getList(1, 50, { sort: '-created' })
      .then((res) => (items = res.items))
      .catch((err) => (error = errorMessage(err)));

    const unsubscribe = collection.subscribe('*', ({ action, record }) => {
      if (action === 'create') items = [record, ...items.filter((i) => i.id !== record.id)];
      if (action === 'delete') items = items.filter((i) => i.id !== record.id);
    });
    return () => unsubscribe.then((stop) => stop());
  });

  async function add(e: SubmitEvent) {
    e.preventDefault();
    error = '';
    try {
      await collection.create({ title });
      title = '';
    } catch (err) {
      error = errorMessage(err);
    }
  }

  async function remove(item: Item) {
    try {
      await collection.delete(item.id);
    } catch (err) {
      error = errorMessage(err);
    }
  }

  const time = (iso: string) =>
    new Date(iso.replace(' ', 'T')).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
</script>

<section class="card flex flex-col gap-3">
  <h2 class="font-semibold">Items <span class="text-xs font-normal text-zinc-400">demo</span></h2>
  <form class="flex gap-2" onsubmit={add}>
    <input placeholder="Add an item" bind:value={title} required maxlength="200" />
    <button class="btn-primary shrink-0">Add</button>
  </form>
  {#if error}<p class="text-sm text-red-600 dark:text-red-400" role="alert">{error}</p>{/if}
  {#if items.length}
    <ul class="divide-y divide-zinc-200 dark:divide-zinc-800">
      {#each items as item (item.id)}
        <li class="flex items-center gap-3 py-2">
          <div class="min-w-0 flex-1">
            <p class="truncate">{item.title}</p>
            <p class="text-xs text-zinc-500">{item.by} · {time(item.created)}</p>
          </div>
          <button
            class="text-sm text-zinc-400 hover:text-red-600"
            aria-label="Delete {item.title}"
            onclick={() => remove(item)}>✕</button
          >
        </li>
      {/each}
    </ul>
  {:else}
    <p class="text-sm text-zinc-500">
      Nothing yet. Add one here, or run the demo worker (see workers/README.md).
    </p>
  {/if}
</section>
