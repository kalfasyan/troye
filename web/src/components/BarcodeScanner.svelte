<script lang="ts">
  import { onMount } from 'svelte';

  // Reads a barcode with the camera where the browser can (Chrome on Android),
  // and always offers typing the number from under the barcode.
  let { onfound }: { onfound: (code: string) => void } = $props();

  type Detector = { detect(source: HTMLVideoElement): Promise<{ rawValue: string }[]> };
  const BarcodeDetector = (
    globalThis as unknown as {
      BarcodeDetector?: new (o: { formats: string[] }) => Detector;
    }
  ).BarcodeDetector;

  let video = $state<HTMLVideoElement>();
  let typed = $state('');
  let message = $state(
    BarcodeDetector
      ? ''
      : "This browser can't read barcodes with the camera. Type the number instead.",
  );

  onMount(() => {
    if (!BarcodeDetector) return;
    let stream: MediaStream | undefined;
    let stopped = false;
    const detector = new BarcodeDetector({ formats: ['ean_13', 'ean_8', 'upc_a', 'upc_e'] });

    (async () => {
      try {
        stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: 'environment' },
        });
        if (stopped) return stream.getTracks().forEach((t) => t.stop());
        video!.srcObject = stream;
        await video!.play();
        while (!stopped) {
          const [hit] = await detector.detect(video!).catch(() => []);
          if (hit?.rawValue) {
            navigator.vibrate?.(50);
            onfound(hit.rawValue);
            return;
          }
          await new Promise((r) => setTimeout(r, 250));
        }
      } catch {
        message = "Couldn't open the camera. Type the number instead.";
      }
    })();

    return () => {
      stopped = true;
      stream?.getTracks().forEach((t) => t.stop());
    };
  });

  function submit(e: SubmitEvent) {
    e.preventDefault();
    const code = typed.replace(/\D/g, '');
    if (code.length >= 6) onfound(code);
  }
</script>

{#if BarcodeDetector && !message}
  <div class="relative overflow-hidden rounded-2xl bg-black">
    <!-- svelte-ignore a11y_media_has_caption -->
    <video bind:this={video} class="aspect-[4/3] w-full object-cover" playsinline muted></video>
    <div
      class="pointer-events-none absolute inset-x-8 top-1/2 h-24 -translate-y-1/2 rounded-xl border-2 border-white/80"
    ></div>
  </div>
  <p class="mt-2 text-center text-sm text-zinc-500 dark:text-zinc-400">
    Point the camera at the barcode.
  </p>
{:else}
  <p class="text-sm text-zinc-600 dark:text-zinc-400">{message}</p>
{/if}

<form class="mt-4 flex gap-2" onsubmit={submit}>
  <input
    inputmode="numeric"
    autocomplete="off"
    placeholder="Barcode number"
    aria-label="Barcode number"
    bind:value={typed}
  />
  <button class="btn-secondary shrink-0">Look up</button>
</form>
