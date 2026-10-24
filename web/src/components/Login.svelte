<script lang="ts">
  import { errorMessage } from '../lib/pb';
  import { session } from '../lib/session.svelte';
  import Brand from './Brand.svelte';

  let email = $state('');
  let password = $state('');
  let busy = $state(false);
  let error = $state('');

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = '';
    try {
      await session.login(email, password);
    } catch (err) {
      const status = (err as { status?: number }).status;
      error = status === 400 ? 'Wrong email or password.' : errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<main class="mx-auto flex min-h-dvh max-w-sm flex-col justify-center px-6 py-10">
  <Brand title="plasto" subtitle="Sign in to continue." />
  <form class="flex flex-col gap-4" onsubmit={submit}>
    <label>Email <input type="email" autocomplete="username" bind:value={email} required /></label>
    <label>
      Password
      <input type="password" autocomplete="current-password" bind:value={password} required />
    </label>
    {#if error}<p class="text-sm text-red-600 dark:text-red-400" role="alert">{error}</p>{/if}
    <button class="btn-primary" disabled={busy}>{busy ? 'Signing in…' : 'Sign in'}</button>
  </form>
</main>
