<script lang="ts">
  import { onMount } from 'svelte';
  import { amountOf, meals, portions, type Food, type Meal } from '../lib/diary';
  import { diary, lookupBarcode, saveCustomFood, searchFoods } from '../lib/diary.svelte';
  import { errorMessage } from '../lib/pb';
  import { photos } from '../lib/photos.svelte';
  import BarcodeScanner from './BarcodeScanner.svelte';
  import Sheet from './Sheet.svelte';

  // Adding food to a meal: search (recent foods first), scan a barcode, make a
  // custom food, or quick-add calories. Picking a food asks how much.
  let { meal: startMeal, onclose }: { meal: Meal; onclose: () => void } = $props();

  type View = 'search' | 'amount' | 'scan' | 'custom' | 'quick' | 'photo';
  let view = $state<View>('search');
  // the meal it opened for, then whichever the person picks
  // svelte-ignore state_referenced_locally
  let meal = $state<Meal>(startMeal);
  let query = $state('');
  let results = $state<Food[]>([]);
  let searching = $state(false);
  let error = $state('');
  // why the new-food form opened after a scan
  let notice = $state('');
  let busy = $state(false);

  // the amount step
  let food = $state<Food | null>(null);
  let grams = $state<number | undefined>(100);
  const preview = $derived(food && grams ? amountOf(food, grams) : null);

  // the custom food and quick add forms
  let custom = $state({
    name: '',
    brand: '',
    kcal: undefined as number | undefined,
    protein: undefined as number | undefined,
    carbs: undefined as number | undefined,
    fat: undefined as number | undefined,
    serving_g: undefined as number | undefined,
    serving_label: '',
  });
  let quick = $state({ name: '', kcal: undefined as number | undefined });

  // the photo step: the picked photo and a note for the analyser
  let photoFile = $state<File | null>(null);
  let photoPreview = $state('');
  let photoNote = $state('');

  function pickedPhoto(e: Event) {
    const file = (e.currentTarget as HTMLInputElement).files?.[0];
    if (!file) return;
    if (photoPreview) URL.revokeObjectURL(photoPreview);
    photoFile = file;
    photoPreview = URL.createObjectURL(file);
    error = '';
    view = 'photo';
  }

  async function sendPhoto() {
    if (!photoFile) return;
    busy = true;
    error = '';
    try {
      await photos.add(photoFile, meal, diary.day, photoNote.trim());
      onclose();
    } catch (err) {
      error = errorMessage(err, "Couldn't send the photo. Please try again.");
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    diary.loadRecent();
  });

  let timer: ReturnType<typeof setTimeout>;
  $effect(() => {
    const q = query;
    clearTimeout(timer);
    if (q.trim().length < 2) {
      results = [];
      return;
    }
    searching = true;
    timer = setTimeout(async () => {
      try {
        results = await searchFoods(q);
        error = '';
      } catch (err) {
        if ((err as { isAbort?: boolean }).isAbort) return;
        error = errorMessage(err);
      } finally {
        searching = false;
      }
    }, 250);
  });

  function pick(f: Food) {
    food = f;
    grams = f.serving_g || 100;
    error = '';
    view = 'amount';
  }

  async function add() {
    if (!food || !grams) return;
    busy = true;
    error = '';
    try {
      await diary.add(meal, food, grams);
      onclose();
    } catch (err) {
      error = errorMessage(err, "Couldn't add it. Please try again.");
    } finally {
      busy = false;
    }
  }

  async function scanned(code: string) {
    busy = true;
    error = '';
    try {
      pick(await lookupBarcode(code));
    } catch (err) {
      notice = errorMessage(err);
      custom = { ...custom, name: '' };
      barcode = code;
      view = 'custom';
    } finally {
      busy = false;
    }
  }
  let barcode = $state('');

  async function saveCustom(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = '';
    try {
      const f = await saveCustomFood({
        name: custom.name.trim(),
        brand: custom.brand.trim(),
        kcal: custom.kcal ?? 0,
        protein: custom.protein ?? 0,
        carbs: custom.carbs ?? 0,
        fat: custom.fat ?? 0,
        serving_g: custom.serving_g ?? 0,
        serving_label: custom.serving_label.trim(),
        barcode,
      });
      pick(f);
    } catch (err) {
      error = errorMessage(err, "Couldn't save the food. Please try again.");
    } finally {
      busy = false;
    }
  }

  async function saveQuick(e: SubmitEvent) {
    e.preventDefault();
    if (!quick.kcal) return;
    busy = true;
    error = '';
    try {
      await diary.quickAdd(meal, quick.name.trim() || 'Quick add', quick.kcal);
      onclose();
    } catch (err) {
      error = errorMessage(err, "Couldn't add it. Please try again.");
    } finally {
      busy = false;
    }
  }

  const titles: Record<View, string> = {
    search: 'Add food',
    amount: 'How much?',
    scan: 'Scan a barcode',
    custom: 'New food',
    quick: 'Quick add',
    photo: 'Meal photo',
  };
  const shown = $derived(query.trim().length >= 2 ? results : diary.recent);
</script>

{#snippet foodRow(f: Food)}
  <li>
    <button
      class="flex w-full items-center justify-between gap-3 px-1 py-3 text-left"
      onclick={() => pick(f)}
    >
      <span class="min-w-0">
        <span class="block truncate font-medium">{f.name}</span>
        <span class="block truncate text-sm text-zinc-500 dark:text-zinc-400">
          {[f.brand, f.serving_g ? f.serving_label || `${f.serving_g} g` : '100 g']
            .filter(Boolean)
            .join(' · ')}
        </span>
      </span>
      <span class="shrink-0 text-sm text-zinc-500 tabular-nums dark:text-zinc-400">
        {Math.round(f.serving_g ? (f.kcal * f.serving_g) / 100 : f.kcal)} kcal
      </span>
    </button>
  </li>
{/snippet}

<Sheet title={titles[view]} onclose={view === 'search' ? onclose : () => (view = 'search')}>
  <div class="mb-4 flex gap-1 overflow-x-auto" role="radiogroup" aria-label="Meal">
    {#each meals as m (m.id)}
      <button
        class="chip shrink-0 py-1.5"
        class:chosen={meal === m.id}
        role="radio"
        aria-checked={meal === m.id}
        onclick={() => (meal = m.id)}>{m.label}</button
      >
    {/each}
  </div>

  {#if error}
    <p class="mb-3 text-sm text-red-600 dark:text-red-400" role="alert">{error}</p>
  {/if}

  {#if view === 'search'}
    <input
      type="search"
      placeholder="Search foods"
      aria-label="Search foods"
      autocomplete="off"
      bind:value={query}
    />
    <label class="btn-primary mt-3 cursor-pointer flex-row gap-2">
      <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="1.8"
        ><path
          d="M4 8h3l2-3h6l2 3h3v11H4z M12 17a4 4 0 1 0 0-8 4 4 0 0 0 0 8"
          stroke-linejoin="round"
        /></svg
      >
      Take a photo
      <input
        class="sr-only"
        type="file"
        accept="image/*"
        capture="environment"
        onchange={pickedPhoto}
      />
    </label>
    <div class="mt-2 grid grid-cols-3 gap-2 text-sm">
      <button class="btn-secondary px-2" onclick={() => (view = 'scan')}>Scan barcode</button>
      <button
        class="btn-secondary px-2"
        onclick={() => ((custom.name = query), (barcode = ''), (view = 'custom'))}>New food</button
      >
      <button class="btn-secondary px-2" onclick={() => (view = 'quick')}>Quick add</button>
    </div>

    <h3 class="mt-5 text-sm font-medium text-zinc-500 dark:text-zinc-400">
      {query.trim().length >= 2 ? (searching ? 'Searching…' : 'Results') : 'Recent'}
    </h3>
    {#if shown.length}
      <ul class="divide-y divide-zinc-200 dark:divide-zinc-800">
        {#each shown as f (f.id)}{@render foodRow(f)}{/each}
      </ul>
    {:else if query.trim().length >= 2 && !searching}
      <p class="mt-2 text-sm text-zinc-500 dark:text-zinc-400">
        Nothing found. Try another word, or add it as a new food.
      </p>
    {:else if query.trim().length < 2}
      <p class="mt-2 text-sm text-zinc-500 dark:text-zinc-400">
        Foods you log show up here, so the next time is one tap.
      </p>
    {/if}
  {:else if view === 'amount' && food}
    <div class="card">
      <p class="font-semibold">{food.name}</p>
      {#if food.brand}<p class="text-sm text-zinc-500 dark:text-zinc-400">{food.brand}</p>{/if}
      <p class="mt-1 text-sm text-zinc-500 dark:text-zinc-400">{food.kcal} kcal per 100 g</p>
    </div>
    <label class="mt-4">
      Amount (g)
      <input type="number" inputmode="decimal" min="1" max="5000" bind:value={grams} />
    </label>
    <div class="mt-2 flex flex-wrap gap-2">
      {#each portions(food) as p (p.label)}
        <button class="chip" class:chosen={grams === p.grams} onclick={() => (grams = p.grams)}
          >{p.label}</button
        >
      {/each}
    </div>
    {#if preview}
      <div class="card mt-4 grid grid-cols-4 gap-2 text-center text-sm">
        <div><b class="block text-lg tabular-nums">{preview.kcal}</b>kcal</div>
        <div><b class="block text-lg tabular-nums">{preview.carbs}</b>carbs</div>
        <div><b class="block text-lg tabular-nums">{preview.protein}</b>protein</div>
        <div><b class="block text-lg tabular-nums">{preview.fat}</b>fat</div>
      </div>
    {/if}
    <button class="btn-primary mt-auto w-full" disabled={!grams || busy} onclick={add}>
      {busy ? 'Adding…' : `Add to ${meals.find((m) => m.id === meal)?.label.toLowerCase()}`}
    </button>
  {:else if view === 'scan'}
    {#if busy}
      <p class="text-center text-zinc-500 dark:text-zinc-400">Looking it up…</p>
    {:else}
      <BarcodeScanner onfound={scanned} />
    {/if}
  {:else if view === 'custom'}
    <form class="flex flex-1 flex-col gap-3" onsubmit={saveCustom}>
      {#if barcode}
        <p class="text-sm text-zinc-600 dark:text-zinc-400">
          {notice} Copy the values from the label once, and the next scan of {barcode} finds it.
        </p>
      {/if}
      <label>Name <input required maxlength="200" bind:value={custom.name} /></label>
      <label>Brand (optional) <input maxlength="200" bind:value={custom.brand} /></label>
      <p class="text-sm font-medium">Per 100 g</p>
      <div class="grid grid-cols-2 gap-3">
        <label
          >Calories <input
            type="number"
            inputmode="decimal"
            min="0"
            max="1000"
            step="any"
            required
            bind:value={custom.kcal}
          /></label
        >
        <label
          >Carbs (g) <input
            type="number"
            inputmode="decimal"
            min="0"
            max="100"
            step="any"
            bind:value={custom.carbs}
          /></label
        >
        <label
          >Protein (g) <input
            type="number"
            inputmode="decimal"
            min="0"
            max="100"
            step="any"
            bind:value={custom.protein}
          /></label
        >
        <label
          >Fat (g) <input
            type="number"
            inputmode="decimal"
            min="0"
            max="100"
            step="any"
            bind:value={custom.fat}
          /></label
        >
      </div>
      <div class="grid grid-cols-2 gap-3">
        <label
          >Serving (g) <input
            type="number"
            inputmode="decimal"
            min="0"
            max="5000"
            step="any"
            bind:value={custom.serving_g}
          /></label
        >
        <label
          >Serving name <input
            maxlength="50"
            placeholder="1 slice"
            bind:value={custom.serving_label}
          /></label
        >
      </div>
      <button class="btn-primary mt-auto" disabled={busy}>{busy ? 'Saving…' : 'Save food'}</button>
    </form>
  {:else if view === 'photo'}
    <div class="flex flex-1 flex-col gap-3">
      {#if photoPreview}
        <img src={photoPreview} alt="Your meal" class="max-h-72 w-full rounded-2xl object-cover" />
      {/if}
      <label
        >Anything it can't see? (optional)
        <input
          maxlength="200"
          placeholder="Cooked in butter, no sauce"
          bind:value={photoNote}
        /></label
      >
      <p class="text-sm text-zinc-600 dark:text-zinc-400">
        Your PC names what's on the plate and weighs it by eye, and the calories come from the food
        tables. You check it before it goes in your diary.
      </p>
      <button class="btn-primary mt-auto" disabled={busy} onclick={sendPhoto}
        >{busy ? 'Sending…' : 'Analyse photo'}</button
      >
    </div>
  {:else if view === 'quick'}
    <form class="flex flex-1 flex-col gap-3" onsubmit={saveQuick}>
      <p class="text-sm text-zinc-600 dark:text-zinc-400">
        For when you just know the calories, like a slice of cake at a party.
      </p>
      <label
        >Calories <input
          type="number"
          inputmode="numeric"
          min="1"
          max="20000"
          required
          bind:value={quick.kcal}
        /></label
      >
      <label>What was it? (optional) <input maxlength="200" bind:value={quick.name} /></label>
      <button class="btn-primary mt-auto" disabled={busy || !quick.kcal}
        >{busy ? 'Adding…' : 'Add'}</button
      >
    </form>
  {/if}
</Sheet>
