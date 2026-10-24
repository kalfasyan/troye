<script lang="ts">
  let { title, subtitle }: { title: string; subtitle: string } = $props();
</script>

<header class="mb-8 text-center">
  <img src="/icons/icon.svg" alt="" class="mx-auto mb-4 h-16 w-16" />
  <h1 class="text-3xl font-bold tracking-tight">{title}</h1>
  <p class="mt-1 text-zinc-500 dark:text-zinc-400">{subtitle}</p>
</header>
