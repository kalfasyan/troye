<script lang="ts">
  import { ago } from '../lib/photos';
  import { photos } from '../lib/photos.svelte';
  import { session } from '../lib/session.svelte';
  import Sheet from './Sheet.svelte';

  // How to get the photo analyser going on the GPU PC (docs/deploy.md, "The
  // photo analyser"), with this app's address filled in. It turns green here
  // as soon as the analyser first asks for work.
  let { onclose }: { onclose: () => void } = $props();

  const run = `docker run -d --name analyser --restart unless-stopped --network host \\
  -e TROYE_URL=${location.origin} \\
  -e TROYE_WORKER=analyser -e TROYE_WORKER_PASSWORD=<the password> \\
  -e OLLAMA_URL=http://127.0.0.1:11434 \\
  ghcr.io/selfhostit/troye-worker python analyser.py`;

  $effect(() => {
    photos.checkAnalyser();
    const every = setInterval(() => photos.checkAnalyser(), 10_000);
    return () => clearInterval(every);
  });
</script>

<Sheet title="Photo analysis" {onclose}>
  <div class="flex flex-col gap-4 text-sm">
    <div class="card flex items-center gap-3">
      <span
        class="size-2.5 shrink-0 rounded-full {photos.analyser.online
          ? 'bg-emerald-500'
          : 'bg-zinc-400'}"
        aria-hidden="true"
      ></span>
      <p>
        {#if photos.analyser.online}
          <b>Your PC is connected.</b> Waiting photos are being analysed now.
        {:else if photos.analyser.seen}
          Your PC was last seen {ago(photos.analyser.seen)}.
        {:else}
          No PC has connected since the app last started.
        {/if}
      </p>
    </div>

    <p class="text-zinc-600 dark:text-zinc-400">
      Photos are analysed by a vision model on your own GPU PC, so they never leave home. The photo
      waits safely in Troye until the PC asks for it, then you get a notification to check what it
      found before anything goes in your diary.
    </p>

    {#if session.isOwner}
      <ol class="flex list-decimal flex-col gap-3 pl-5">
        <li>
          On the PC, install <a
            class="text-brand-600 underline dark:text-brand-400"
            href="https://ollama.com/download"
            target="_blank"
            rel="noreferrer">Ollama</a
          >
          and download the model:
          <code class="mt-1 block rounded-lg bg-zinc-100 px-3 py-2 dark:bg-zinc-800"
            >ollama pull qwen2.5vl:7b</code
          >
        </li>
        <li>
          In the <a
            class="text-brand-600 underline dark:text-brand-400"
            href="/_/"
            target="_blank"
            rel="noreferrer">admin page</a
          >, open <b>workers</b> and add a record named <code>analyser</code> with a password.
        </li>
        <li>
          Make sure the PC can open this address: <code class="break-all">{location.origin}</code>.
          If you reach Troye through Tailscale, install Tailscale on the PC and sign in to the same
          account.
        </li>
        <li>
          Start the analyser on the PC with Docker, so it starts with the PC:
          <pre
            class="mt-1 overflow-x-auto rounded-lg bg-zinc-100 px-3 py-2 text-xs dark:bg-zinc-800">{run}</pre>
          Without Docker, run this in the<code>workers</code> folder of the Troye code:
          <code>uv run python analyser.py</code> with the same settings.
        </li>
        <li>Within half a minute, this page says your PC is connected.</li>
      </ol>
    {:else}
      <p>The photo analyser runs on the owner's PC. Ask them to set it up or turn the PC on.</p>
    {/if}
  </div>
  <button class="btn-primary mt-6" onclick={onclose}>Done</button>
</Sheet>
