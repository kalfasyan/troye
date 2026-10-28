<script lang="ts">
  import { onMount } from 'svelte';
  import {
    addOnIdeas,
    amountOf,
    entryAmount,
    foodName,
    meals,
    type Entry,
    type Food,
    type Meal,
  } from '../lib/diary';
  import { diary, lookupBarcode, saveCustomFood, searchFoods } from '../lib/diary.svelte';
  import { errorMessage } from '../lib/pb';
  import { statusText, type MealPhoto } from '../lib/photos';
  import { photos } from '../lib/photos.svelte';
  import { startAmount } from '../lib/portions';
  import { profile } from '../lib/profile.svelte';
  import AnalyserCard from './AnalyserCard.svelte';
  import BarcodeScanner from './BarcodeScanner.svelte';
  import Sheet from './Sheet.svelte';
  import AmountPicker from './AmountPicker.svelte';

  // Adding food to a meal: search (recent foods first), take a photo, scan a
  // barcode, make a custom food, or quick-add calories. Picking a food asks how
  // much. A photo is sent the moment it is taken, so it is never lost, and the
  // sheet then says where it is and whether the PC is there to analyse it.
  // Once a food is in, the sheet offers what usually goes with it (milk for a
  // coffee, an egg for a noodle soup), and those are logged as its add-ons.
  let {
    meal: startMeal,
    addTo,
    query: startQuery = '',
    onclose,
    onreview,
  }: {
    meal: Meal;
    /** opens it to add something to this entry */
    addTo?: Entry;
    query?: string;
    onclose: () => void;
    onreview: (p: MealPhoto) => void;
  } = $props();

  type View = 'search' | 'amount' | 'scan' | 'custom' | 'quick' | 'photo' | 'added';
  let view = $state<View>('search');
  // the meal it opened for, then whichever the person picks
  // svelte-ignore state_referenced_locally
  let meal = $state<Meal>(startMeal);
  // svelte-ignore state_referenced_locally
  let query = $state(startQuery);
  // the entry being added to: given, or the food just logged
  // svelte-ignore state_referenced_locally
  let parent = $state<Entry | null>(addTo ?? null);
  // the parent as it is now in the diary, with what was added to it
  const base = $derived(parent ? (diary.entries.find((e) => e.id === parent!.id) ?? parent) : null);
  const baseAddOns = $derived(base ? diary.addOnsOf(base) : []);
  let usual = $state<Food[]>([]);
  const ideas = $derived(
    base
      ? addOnIdeas(base.name).filter(
          (w) =>
            ![...usual.map(name), ...baseAddOns.map((a) => a.name)].some((n) =>
              n.toLowerCase().includes(w),
            ),
        )
      : [],
  );
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
  // the amount as counted, such as "2 tbsp", or empty for grams
  let amount = $state('');
  const name = (f: Food) => foodName(f, profile.current?.food_language);

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

  // the photo step: the picked photo, the queued record, and a note for the analyser
  let photoFile = $state<File | null>(null);
  let photoPreview = $state('');
  let photoNote = $state('');
  let sent = $state<MealPhoto | null>(null);
  // the queued photo as it changes, so the sheet shows it being analysed
  const live = $derived(sent ? (photos.pending.find((p) => p.id === sent!.id) ?? sent) : null);

  function pickedPhoto(e: Event) {
    const file = (e.currentTarget as HTMLInputElement).files?.[0];
    if (!file) return;
    if (photoPreview) URL.revokeObjectURL(photoPreview);
    photoFile = file;
    photoPreview = URL.createObjectURL(file);
    sent = null;
    view = 'photo';
    sendPhoto();
  }

  async function sendPhoto() {
    if (!photoFile) return;
    busy = true;
    error = '';
    try {
      sent = await photos.add(photoFile, meal, diary.day, '');
    } catch (err) {
      error = errorMessage(err, "Couldn't send the photo. Please try again.");
    } finally {
      busy = false;
    }
  }

  /** Saves a changed note or meal on the sent photo, then closes. */
  async function donePhoto() {
    const p = live;
    const note = photoNote.trim();
    if (p && p.status !== 'logged' && (note !== p.note || meal !== p.meal)) {
      busy = true;
      try {
        await photos.edit(p, { note, meal });
      } catch (err) {
        error = errorMessage(err, "Couldn't save the note.");
        busy = false;
        return;
      }
    }
    onclose();
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
    ({ grams, amount } = startAmount(f, diary.lastAmounts.get(f.id)));
    error = '';
    view = 'amount';
  }

  async function add() {
    if (!food || !grams) return;
    busy = true;
    error = '';
    try {
      const e = await diary.add(meal, food, grams, amount, parent ?? undefined);
      await added(e);
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
      const e = await diary.quickAdd(
        meal,
        quick.name.trim() || 'Quick add',
        quick.kcal,
        parent ?? undefined,
      );
      quick = { name: '', kcal: undefined };
      await added(e);
    } catch (err) {
      error = errorMessage(err, "Couldn't add it. Please try again.");
    } finally {
      busy = false;
    }
  }

  /**
   * After logging: an add-on goes back to its entry, so more can go with it; a
   * food that things usually go with offers them; anything else is done.
   */
  async function added(e: Entry) {
    if (parent) {
      showAdded(parent);
      return;
    }
    const before = e.food ? await diary.usualAddOns(e.food) : [];
    if (!before.length && !addOnIdeas(e.name).length) {
      onclose();
      return;
    }
    showAdded(e, before);
  }

  async function showAdded(e: Entry, known?: Food[]) {
    parent = e;
    query = '';
    error = '';
    view = 'added';
    usual = known ?? (e.food ? await diary.usualAddOns(e.food) : []);
  }

  function addOn(words: string) {
    query = words;
    view = 'search';
  }

  const titles: Record<View, string> = {
    added: 'Added',
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
  {@const usual = startAmount(f, diary.lastAmounts.get(f.id))}
  <li>
    <button
      class="flex w-full items-center justify-between gap-3 px-1 py-3 text-left"
      onclick={() => pick(f)}
    >
      <span class="min-w-0">
        <span class="block truncate font-medium">{name(f)}</span>
        <span class="block truncate text-sm text-zinc-500 dark:text-zinc-400">
          {[f.brand, usual.amount || `${usual.grams} g`].filter(Boolean).join(' · ')}
        </span>
      </span>
      <span class="shrink-0 text-sm text-zinc-500 tabular-nums dark:text-zinc-400">
        {Math.round((f.kcal * usual.grams) / 100)} kcal
      </span>
    </button>
  </li>
{/snippet}

<Sheet
  title={view === 'photo' && sent
    ? 'Photo received'
    : base && view === 'search'
      ? `Add to ${base.name}`
      : addTo && view === 'added'
        ? addTo.name
        : titles[view]}
  onclose={view === 'added' || (view === 'search' && !base)
    ? onclose
    : view === 'search'
      ? () => showAdded(base!)
      : view === 'photo' && sent
        ? donePhoto
        : () => (view = 'search')}
>
  {#if !base}
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
  {/if}

  {#if error}
    <p class="mb-3 text-sm text-red-600 dark:text-red-400" role="alert">{error}</p>
  {/if}

  {#if view === 'added' && base}
    <div class="flex flex-1 flex-col gap-4">
      <div class="card flex items-center gap-4">
        <img src="/brand/plato-cheer.svg" alt="" class="size-12 shrink-0" />
        <div class="min-w-0 flex-1">
          <p class="truncate font-semibold">{base.name}</p>
          <p class="text-sm text-zinc-500 dark:text-zinc-400">
            {[entryAmount(base), meals.find((m) => m.id === base.meal)?.label]
              .filter(Boolean)
              .join(' · ')}
          </p>
        </div>
        <span class="shrink-0 font-semibold tabular-nums">
          {Math.round(base.kcal + baseAddOns.reduce((s, a) => s + a.kcal, 0))} kcal
        </span>
      </div>
      {#if baseAddOns.length}
        <ul class="-mt-2 space-y-1 px-1 text-sm text-zinc-600 dark:text-zinc-400">
          {#each baseAddOns as a (a.id)}
            <li class="flex justify-between gap-3">
              <span class="truncate">+ {a.name} · {entryAmount(a)}</span>
              <span class="shrink-0 tabular-nums">{Math.round(a.kcal)}</span>
            </li>
          {/each}
        </ul>
      {/if}

      <div>
        <h3 class="text-sm font-medium">Anything with it?</h3>
        <div class="mt-2 flex flex-wrap gap-2">
          {#each usual
            .filter((f) => !baseAddOns.some((a) => a.food === f.id))
            .slice(0, 4) as f (f.id)}
            <button class="chip" onclick={() => pick(f)}>+ {name(f)}</button>
          {/each}
          {#each ideas.slice(0, 6 - Math.min(usual.length, 4)) as idea (idea)}
            <button class="chip" onclick={() => addOn(idea)}>+ {idea}</button>
          {/each}
          <button
            class="chip font-medium text-brand-600 dark:text-brand-400"
            onclick={() => addOn('')}>+ Something else</button
          >
        </div>
      </div>

      <button class="btn-primary mt-auto" onclick={onclose}>Done</button>
    </div>
  {:else if view === 'search'}
    <!-- svelte-ignore a11y_autofocus -->
    <input
      type="search"
      placeholder={base ? `What did you add to ${base.name.toLowerCase()}?` : 'Search foods'}
      aria-label="Search foods"
      autocomplete="off"
      autofocus={!!base}
      bind:value={query}
    />
    {#if !base}
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
    {/if}
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
      <p class="font-semibold">{name(food)}</p>
      {#if food.brand}<p class="text-sm text-zinc-500 dark:text-zinc-400">{food.brand}</p>{/if}
      <p class="mt-1 text-sm text-zinc-500 dark:text-zinc-400">{food.kcal} kcal per 100 g</p>
    </div>
    <div class="mt-4 flex flex-col gap-3">
      <AmountPicker {food} bind:grams bind:amount />
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
      {busy
        ? 'Adding…'
        : `Add to ${base ? base.name.toLowerCase() : meals.find((m) => m.id === meal)?.label.toLowerCase()}`}
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
        <img src={photoPreview} alt="Your meal" class="max-h-64 w-full rounded-2xl object-cover" />
      {/if}
      {#if live}
        <div class="card flex items-center gap-4" role="status">
          <img
            src={live.status === 'ready'
              ? '/brand/plato-cheer.svg'
              : live.status === 'failed'
                ? '/brand/plato.svg'
                : '/brand/plato-thinking.svg'}
            alt=""
            class="size-14 shrink-0"
          />
          <div class="min-w-0">
            <p class="font-medium">{statusText(live, photos.analyser.online)}</p>
            <p class="text-sm text-zinc-500 dark:text-zinc-400">
              {#if live.status === 'ready'}
                Check what it found before it goes in your diary.
              {:else if live.status === 'failed'}
                {live.error}
              {:else}
                It's saved under {meals.find((m) => m.id === live.meal)?.label.toLowerCase()} in your
                diary. You'll get a notification when it's ready to check.
              {/if}
            </p>
          </div>
        </div>
        {#if live.status === 'ready' || live.status === 'failed'}
          <button class="btn-primary" onclick={() => onreview(live)}>Check it now</button>
        {:else}
          <AnalyserCard always />
          <label
            >Anything it can't see? (optional)
            <input
              maxlength="200"
              placeholder="Cooked in butter, no sauce"
              bind:value={photoNote}
            /></label
          >
        {/if}
        <button class="btn-secondary mt-auto" disabled={busy} onclick={donePhoto}>Done</button>
      {:else if busy}
        <p class="text-sm text-zinc-600 dark:text-zinc-400" role="status">Sending your photo…</p>
      {:else}
        <button class="btn-primary mt-auto" onclick={sendPhoto}>Try again</button>
      {/if}
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
