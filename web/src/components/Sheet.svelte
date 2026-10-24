<script lang="ts">
  import type { Snippet } from 'svelte';

  // A full-screen panel over the app, with a title and a close button.
  // Escape closes it.
  let { title, onclose, children }: { title: string; onclose: () => void; children: Snippet } =
    $props();

  $effect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onclose();
    addEventListener('keydown', onKey);
    document.body.style.overflow = 'hidden';
    return () => {
      removeEventListener('keydown', onKey);
      document.body.style.overflow = '';
    };
  });
</script>

<div
  class="fixed inset-0 z-20 overflow-y-auto bg-zinc-50 dark:bg-zinc-950"
  role="dialog"
  aria-modal="true"
  aria-label={title}
>
  <div
    class="mx-auto flex min-h-full max-w-md flex-col px-4 pt-[max(1rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))]"
  >
    <header class="mb-4 flex items-center gap-2">
      <button
        class="-ml-2 rounded-full p-2 text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800"
        aria-label="Close"
        onclick={onclose}
      >
        <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2"
          ><path d="M6 6l12 12M18 6L6 18" stroke-linecap="round" /></svg
        >
      </button>
      <h2 class="text-lg font-semibold">{title}</h2>
    </header>
    {@render children()}
  </div>
</div>
