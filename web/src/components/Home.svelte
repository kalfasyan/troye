<script lang="ts">
  import { onMount } from 'svelte';
  import Diary from './Diary.svelte';
  import Me from './Me.svelte';
  import Photos from './Photos.svelte';
  import Weight from './Weight.svelte';

  // The signed-in app: tabs, each with its own #address so Back works.
  const tabs = [
    {
      id: 'diary',
      label: 'Diary',
      icon: 'M12 3a9 9 0 1 0 9 9M12 3a9 9 0 0 1 9 9M12 8a4 4 0 1 0 4 4',
    },
    {
      id: 'photos',
      label: 'Photos',
      icon: 'M4 8h3l2-3h6l2 3h3v11H4zM12 17a4 4 0 1 0 0-8 4 4 0 0 0 0 8',
    },
    {
      id: 'weight',
      label: 'Weight',
      icon: 'M4 20h16M5 16l4.5-4.5 3 3L19 8M15 8h4v4',
    },
    {
      id: 'me',
      label: 'Me',
      icon: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8Zm-7 8a7 7 0 0 1 14 0',
    },
  ] as const;
  type Tab = (typeof tabs)[number]['id'];

  const fromHash = (): Tab => tabs.find((t) => `#${t.id}` === location.hash)?.id ?? 'diary';
  let tab = $state<Tab>(fromHash());

  onMount(() => {
    const onHash = () => (tab = fromHash());
    addEventListener('hashchange', onHash);
    return () => removeEventListener('hashchange', onHash);
  });
</script>

<div class="mx-auto min-h-dvh max-w-md">
  <main class="flex flex-col gap-4 px-4 pt-[max(1.25rem,env(safe-area-inset-top))] pb-28">
    {#if tab === 'diary'}
      <Diary />
    {:else if tab === 'photos'}
      <Photos />
    {:else if tab === 'weight'}
      <Weight />
    {:else}
      <Me />
    {/if}
  </main>

  <nav
    class="fixed inset-x-0 bottom-0 z-10 border-t border-zinc-200 bg-white/90 pb-[env(safe-area-inset-bottom)] backdrop-blur dark:border-zinc-800 dark:bg-zinc-950/90"
  >
    <div class="mx-auto grid max-w-md grid-cols-4">
      {#each tabs as t (t.id)}
        <a
          href="#{t.id}"
          class="flex flex-col items-center gap-0.5 py-2.5 text-xs font-medium {tab === t.id
            ? 'text-brand-600 dark:text-brand-400'
            : 'text-zinc-500'}"
          aria-current={tab === t.id ? 'page' : undefined}
        >
          <svg
            viewBox="0 0 24 24"
            class="size-6"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"><path d={t.icon} /></svg
          >
          {t.label}
        </a>
      {/each}
    </div>
  </nav>
</div>
