<script lang="ts">
  import { onMount } from 'svelte';
  import { tableInfo, tableStatus, type FoodTable, type TableName } from '../lib/foodTables';
  import { errorMessage, pb } from '../lib/pb';
  import { session } from '../lib/session.svelte';
  import Sheet from './Sheet.svelte';

  // Where the foods come from: each table's size and edition, the credit its
  // licence asks for, and for the owner, importing NEVO and updating the
  // Open Food Facts products.
  let { onclose }: { onclose: () => void } = $props();

  let tables = $state<Partial<Record<TableName, FoodTable>>>({});
  let busy = $state(false);
  let error = $state('');
  let fileInput = $state<HTMLInputElement>();

  onMount(() => {
    pb.collection('food_tables')
      .getFullList<FoodTable>({ requestKey: null })
      .then((list) => list.forEach((t) => (tables[t.name] = t)))
      .catch(() => (error = "Couldn't load the food tables."));
    // an import or download in progress updates live
    const off = pb
      .collection('food_tables')
      .subscribe<FoodTable>('*', ({ record }) => (tables[record.name] = record));
    return () => off.then((unsubscribe) => unsubscribe()).catch(() => {});
  });

  async function importNEVO(e: Event) {
    const file = (e.currentTarget as HTMLInputElement).files?.[0];
    if (!file) return;
    busy = true;
    error = '';
    try {
      const body = new FormData();
      body.append('file', file);
      tables.nevo = await pb.send<FoodTable>('/api/app/foods/nevo', { method: 'POST', body });
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
      if (fileInput) fileInput.value = '';
    }
  }

  async function updateProducts() {
    error = '';
    try {
      await pb.send('/api/app/foods/off', { method: 'POST' });
    } catch (err) {
      error = errorMessage(err);
    }
  }

  const order: TableName[] = ['usda', 'nevo', 'off'];
</script>

<Sheet title="Food data" {onclose}>
  <p class="mb-4 text-sm text-zinc-600 dark:text-zinc-400">
    Search looks through these tables in English, Dutch and French. They're stored on your own
    server.
  </p>

  <div class="flex flex-col gap-3">
    {#each order as name (name)}
      {@const info = tableInfo[name]}
      {@const t = tables[name]}
      <section class="card">
        <h3 class="font-semibold">{info.title}</h3>
        <p class="mt-1 text-sm">{info.about}</p>
        <p class="mt-2 text-sm font-medium text-zinc-700 dark:text-zinc-300">{tableStatus(t)}</p>

        {#if name === 'nevo' && !t?.foods && !t?.running}
          <p class="mt-2 text-sm text-zinc-600 dark:text-zinc-400">
            NEVO is free to use, but it can't come with the app. {#if session.isOwner}Download it
              once from the <a
                class="text-brand-600 underline dark:text-brand-400"
                href={info.download ?? info.link}
                target="_blank"
                rel="noreferrer">RIVM website</a
              > after accepting their terms, then choose the CSV or Excel file here.{:else}Ask the
              owner to import it.{/if}
          </p>
        {/if}

        {#if session.isOwner && name === 'nevo'}
          <label
            class="btn-secondary mt-3 block cursor-pointer text-center"
            class:opacity-50={busy}
          >
            {busy ? 'Importing…' : t?.foods ? 'Import a newer NEVO file' : 'Choose the NEVO file'}
            <input
              bind:this={fileInput}
              type="file"
              accept=".csv,.txt,.xlsx,.zip"
              class="sr-only"
              disabled={busy}
              onchange={importNEVO}
            />
          </label>
        {/if}

        {#if session.isOwner && name === 'off'}
          <button class="btn-secondary mt-3 w-full" disabled={t?.running} onclick={updateProducts}>
            {t?.running ? 'Downloading…' : 'Update now'}
          </button>
          <p class="mt-1 text-xs text-zinc-500">
            It updates by itself every month. A download is about 1 GB.
          </p>
        {/if}

        <p class="mt-3 text-xs text-zinc-500">
          Source: <a class="underline" href={info.link} target="_blank" rel="noreferrer"
            >{info.credit}</a
          >
        </p>
      </section>
    {/each}
  </div>

  {#if error}
    <p class="mt-3 text-sm text-red-600 dark:text-red-400" role="alert">{error}</p>
  {/if}
</Sheet>
