<script lang="ts">
  import { onMount } from 'svelte';
  import { dayLabel, isoDay } from '../lib/diary';
  import { diary } from '../lib/diary.svelte';
  import { errorMessage } from '../lib/pb';
  import { analyserText } from '../lib/photos';
  import { photos } from '../lib/photos.svelte';
  import AnalyserSetup from './AnalyserSetup.svelte';

  // Where the PC that analyses photos is, while photos wait for it: on, off
  // (with Wake PC), or never set up (with the steps). It also points to photos
  // waiting on other days, with `otherDays`. `always` shows it with nothing waiting.
  let { always = false, otherDays = false }: { always?: boolean; otherDays?: boolean } = $props();

  const waiting = $derived(
    photos.pending.filter((p) => p.status === 'queued' || p.status === 'analysing'),
  );
  const elsewhere = $derived(otherDays ? photos.pending.filter((p) => p.date !== diary.day) : []);
  const text = $derived(analyserText(photos.analyser, photos.waking));
  let waking = $state(false);
  let error = $state('');
  let setup = $state(false);

  onMount(() => {
    photos.checkAnalyser();
    // check on the PC while photos wait for it
    const every = setInterval(() => {
      if (waiting.length || photos.waking) photos.checkAnalyser();
    }, 20_000);
    return () => clearInterval(every);
  });

  async function wake() {
    waking = true;
    error = '';
    try {
      await photos.wake();
    } catch (err) {
      error = errorMessage(err, "Couldn't wake your PC.");
    } finally {
      waking = false;
    }
  }
</script>

{#if photos.checked && (always || waiting.length || elsewhere.length)}
  <section class="card p-0" aria-live="polite">
    {#if always || waiting.length}
      <div class="flex items-start gap-3 px-4 py-3">
        <span
          class="mt-1.5 size-2.5 shrink-0 rounded-full {photos.analyser.online
            ? 'bg-emerald-500'
            : photos.analyser.set_up
              ? 'bg-zinc-400'
              : 'bg-amber-500'}"
          aria-hidden="true"
        ></span>
        <div class="min-w-0 flex-1">
          <p class="font-medium">{text.title}</p>
          <p class="text-sm text-zinc-500 dark:text-zinc-400">
            {#if waiting.length && !always}
              {waiting.length === 1 ? '1 photo is' : `${waiting.length} photos are`} waiting.
            {/if}
            {text.detail}
          </p>
        </div>
        {#if !photos.analyser.online && !photos.analyser.set_up}
          <button
            class="shrink-0 text-sm font-semibold text-brand-600 dark:text-brand-400"
            onclick={() => (setup = true)}>Set it up</button
          >
        {:else if !photos.analyser.online && photos.analyser.can_wake}
          <button
            class="shrink-0 text-sm font-semibold text-brand-600 dark:text-brand-400"
            disabled={waking}
            onclick={wake}>{photos.waking ? 'Wake again' : 'Wake PC'}</button
          >
        {:else if !photos.analyser.online}
          <button
            class="shrink-0 text-sm font-semibold text-brand-600 dark:text-brand-400"
            onclick={() => (setup = true)}>Help</button
          >
        {/if}
      </div>
    {/if}
    {#if elsewhere.length && !always}
      {@const day = elsewhere[0].date}
      <button
        class="flex w-full items-center justify-between gap-3 border-t border-zinc-100 px-4 py-3 text-left text-sm first:border-t-0 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800/50"
        onclick={() => diary.show(day)}
      >
        <span class="text-zinc-600 dark:text-zinc-400">
          {elsewhere.length === 1 ? '1 photo' : `${elsewhere.length} photos`} from other days
          {elsewhere.length === 1 ? 'is' : 'are'} not in your diary yet
        </span>
        <span class="shrink-0 font-semibold text-brand-600 dark:text-brand-400"
          >{dayLabel(day, isoDay(new Date()))}</span
        >
      </button>
    {/if}
    {#if error}<p class="px-4 pb-3 text-sm text-red-600 dark:text-red-400" role="alert">
        {error}
      </p>{/if}
  </section>
{/if}

{#if setup}
  <AnalyserSetup onclose={() => (setup = false)} />
{/if}
