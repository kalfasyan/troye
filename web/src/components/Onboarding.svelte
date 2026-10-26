<script lang="ts">
  import { errorMessage } from '../lib/pb';
  import {
    activities,
    blankAnswers,
    cmToFtIn,
    diets,
    eatingEnd,
    fastingDaysLabel,
    fastings,
    formatPace,
    formatWeight,
    ftInToCm,
    goals,
    kgToLb,
    lbToKg,
    motivations,
    paces,
    weekdays,
    type Answers,
    type Motivation,
    type Plan,
    type Weekday,
  } from '../lib/profile';
  import { previewPlan, profile } from '../lib/profile.svelte';
  import { session } from '../lib/session.svelte';

  // The onboarding interview: one question a screen, with Plato asking. The
  // answers set the daily calorie budget, macros, fasting window and goal date.
  // Opened again from the home screen, it starts from the saved answers.

  const saved = profile.current;
  let a = $state<Answers>(
    saved
      ? {
          ...blankAnswers(),
          ...Object.fromEntries(
            Object.keys(blankAnswers()).map((k) => [k, saved[k as keyof Answers]]),
          ),
        }
      : blankAnswers(),
  );
  if (!a.fasting) a.fasting = 'none';
  if (!a.diet) a.diet = 'balanced';
  if (!a.units) a.units = 'metric';
  if (!a.eating_start) a.eating_start = '12:00';
  if (!a.pace_kg) a.pace_kg = 0.5;
  a.motivations ??= [];
  a.fasting_days ??= [];

  // no days chosen is every day; the last day can't be taken off
  function toggleDay(day: Weekday) {
    const on = a.fasting_days.length ? a.fasting_days : weekdays.map((d) => d.id);
    const next = on.includes(day) ? on.filter((d) => d !== day) : [...on, day];
    if (!next.length) return;
    a.fasting_days = next.length === weekdays.length ? [] : next;
  }

  const firstName = (session.user?.name || '').split(' ')[0];

  type Step =
    | 'welcome'
    | 'goal'
    | 'motivation'
    | 'about'
    | 'body'
    | 'target'
    | 'activity'
    | 'fasting'
    | 'diet'
    | 'plan';
  const steps = $derived<Step[]>(
    [
      'welcome',
      'goal',
      'motivation',
      'about',
      'body',
      'target',
      'activity',
      'fasting',
      'diet',
      'plan',
    ].filter((s) => s !== 'target' || a.goal !== 'maintain') as Step[],
  );
  let index = $state(0);
  const step = $derived(steps[Math.min(index, steps.length - 1)]);

  // inputs in the person's units; the answers stay metric
  let ft = $state(a.height_cm ? cmToFtIn(a.height_cm)[0] : 0);
  let inch = $state(a.height_cm ? cmToFtIn(a.height_cm)[1] : 0);
  let weightIn = $state(a.weight_kg ? round(shown(a.weight_kg)) : undefined);
  let goalIn = $state(a.goal_weight_kg ? round(shown(a.goal_weight_kg)) : undefined);

  function round(n: number) {
    return Math.round(n * 10) / 10;
  }
  function shown(kg: number) {
    return a.units === 'imperial' ? kgToLb(kg) : kg;
  }
  function fromShown(n: number | undefined) {
    if (!n) return 0;
    return a.units === 'imperial' ? lbToKg(n) : n;
  }
  function setUnits(units: Answers['units']) {
    a.units = units;
    weightIn = a.weight_kg ? round(shown(a.weight_kg)) : undefined;
    goalIn = a.goal_weight_kg ? round(shown(a.goal_weight_kg)) : undefined;
    [ft, inch] = a.height_cm ? cmToFtIn(a.height_cm) : [0, 0];
  }
  $effect(() => {
    a.weight_kg = round(fromShown(weightIn));
  });
  $effect(() => {
    a.goal_weight_kg = round(fromShown(goalIn));
  });
  $effect(() => {
    if (a.units === 'imperial') a.height_cm = round(ft || inch ? ftInToCm(ft, inch) : 0);
  });

  const today = new Date().toISOString().slice(0, 10);
  const weightUnit = $derived(a.units === 'imperial' ? 'lb' : 'kg');

  const canGoOn = $derived.by(() => {
    switch (step) {
      case 'goal':
        return a.goal !== '';
      case 'about':
        return a.sex !== '' && a.birth_date !== '';
      case 'body':
        return a.height_cm >= 100 && a.weight_kg >= 30;
      case 'target':
        return (
          a.goal_weight_kg >= 30 &&
          (a.goal === 'lose' ? a.goal_weight_kg < a.weight_kg : a.goal_weight_kg > a.weight_kg)
        );
      case 'activity':
        return a.activity !== '';
      default:
        return true;
    }
  });

  // a rough goal date for the pace buttons; the plan step shows the real one
  function roughDate(pace: number) {
    const weeks = Math.abs(a.weight_kg - a.goal_weight_kg) / pace;
    const d = new Date(Date.now() + weeks * 7 * 864e5);
    return d.toLocaleDateString(undefined, { month: 'short', year: 'numeric' });
  }

  let plan = $state<Plan | null>(null);
  let planError = $state('');
  let busy = $state(false);
  let saveError = $state('');

  async function next() {
    if (!canGoOn) return;
    index++;
    if (steps[index] === 'plan') {
      plan = null;
      planError = '';
      try {
        plan = await previewPlan(a);
      } catch (err) {
        planError = errorMessage(err);
      }
    }
    window.scrollTo(0, 0);
  }

  function back() {
    if (index > 0) index--;
  }

  function toggle(m: Motivation) {
    a.motivations = a.motivations.includes(m)
      ? a.motivations.filter((x) => x !== m)
      : [...a.motivations, m];
  }

  async function finish() {
    busy = true;
    saveError = '';
    try {
      await profile.save(a);
    } catch (err) {
      saveError = errorMessage(err, "Couldn't save your plan. Please try again.");
    } finally {
      busy = false;
    }
  }

  const goalDate = (d: string) =>
    new Date(d + 'T12:00:00').toLocaleDateString(undefined, {
      day: 'numeric',
      month: 'long',
      year: 'numeric',
    });

  const mascot = $derived(step === 'plan' ? (plan ? 'plato-cheer' : 'plato-thinking') : 'plato');
</script>

<main
  class="mx-auto flex min-h-dvh max-w-md flex-col px-5 pt-[max(1.25rem,env(safe-area-inset-top))] pb-8"
>
  <div class="flex items-center gap-3">
    <button
      class="rounded-full p-2 text-zinc-500 hover:bg-zinc-100 disabled:invisible dark:hover:bg-zinc-800"
      aria-label="Back"
      disabled={index === 0}
      onclick={back}
    >
      <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2"
        ><path d="M15 5l-7 7 7 7" stroke-linecap="round" stroke-linejoin="round" /></svg
      >
    </button>
    <div
      class="h-2 flex-1 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800"
      role="progressbar"
      aria-valuemin="0"
      aria-valuemax={steps.length - 1}
      aria-valuenow={index}
    >
      <div
        class="h-full rounded-full bg-brand-600 transition-all"
        style="width: {(index / (steps.length - 1)) * 100}%"
      ></div>
    </div>
  </div>

  <div class="mt-6 flex items-end gap-3">
    <img
      src="/brand/{mascot}.svg"
      alt=""
      class="size-16 shrink-0"
      onerror={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')}
    />
    <h1 class="card rounded-bl-sm px-4 py-3 text-lg leading-snug font-semibold">
      {#if step === 'welcome'}
        Hi{firstName ? ` ${firstName}` : ''}! I'm Plato. Let's build your plan together.
      {:else if step === 'goal'}
        What would you like to do?
      {:else if step === 'motivation'}
        What's driving you? Pick as many as you like.
      {:else if step === 'about'}
        A little about you, so I can work out what your body burns.
      {:else if step === 'body'}
        How tall are you, and what do you weigh today?
      {:else if step === 'target'}
        Where would you like to get to, and how fast?
      {:else if step === 'activity'}
        How active is a normal day for you?
      {:else if step === 'fasting'}
        Would you like to try intermittent fasting?
      {:else if step === 'diet'}
        How would you like to split your calories?
      {:else if plan}
        Here's your plan. You've got this!
      {:else if planError}
        Something doesn't add up yet.
      {:else}
        Working out your plan…
      {/if}
    </h1>
  </div>

  <section class="mt-6 flex flex-1 flex-col gap-3">
    {#if step === 'welcome'}
      <p class="text-zinc-600 dark:text-zinc-400">
        I'll ask a few quick questions, about two minutes. Your answers set your daily calorie
        budget, your macros and your goal date. They stay on your own server, and you can change
        them any time.
      </p>
    {:else if step === 'goal'}
      {#each goals as g (g.id)}
        <button class="choice" class:chosen={a.goal === g.id} onclick={() => (a.goal = g.id)}>
          <span class="font-semibold">{g.label}</span>
          <span class="text-sm text-zinc-500 dark:text-zinc-400">{g.hint}</span>
        </button>
      {/each}
    {:else if step === 'motivation'}
      <div class="flex flex-wrap gap-2">
        {#each motivations as m (m.id)}
          <button
            class="chip"
            class:chosen={a.motivations.includes(m.id)}
            aria-pressed={a.motivations.includes(m.id)}
            onclick={() => toggle(m.id)}><span aria-hidden="true">{m.emoji}</span> {m.label}</button
          >
        {/each}
      </div>
      <label class="mt-3">
        In your own words (optional)
        <textarea
          rows="3"
          maxlength="500"
          bind:value={a.why}
          placeholder="I want long hikes with my dog to feel easy"></textarea>
      </label>
      <p class="text-sm text-zinc-500 dark:text-zinc-400">
        I'll remind you of this on the hard days.
      </p>
    {:else if step === 'about'}
      <fieldset class="flex flex-col gap-1.5">
        <legend class="mb-1.5 text-sm font-medium">Sex, for the calorie calculation</legend>
        <div class="grid grid-cols-2 gap-2">
          <button
            class="choice"
            class:chosen={a.sex === 'female'}
            onclick={() => (a.sex = 'female')}>Female</button
          >
          <button class="choice" class:chosen={a.sex === 'male'} onclick={() => (a.sex = 'male')}
            >Male</button
          >
        </div>
      </fieldset>
      <label>
        Date of birth
        <input type="date" max={today} bind:value={a.birth_date} />
      </label>
    {:else if step === 'body'}
      <div class="grid grid-cols-2 gap-2">
        <button
          class="choice py-2"
          class:chosen={a.units === 'metric'}
          onclick={() => setUnits('metric')}>kg, cm</button
        >
        <button
          class="choice py-2"
          class:chosen={a.units === 'imperial'}
          onclick={() => setUnits('imperial')}>lb, ft</button
        >
      </div>
      {#if a.units === 'metric'}
        <label>
          Height (cm)
          <input type="number" inputmode="decimal" min="100" max="250" bind:value={a.height_cm} />
        </label>
      {:else}
        <div class="grid grid-cols-2 gap-2">
          <label
            >Height (ft) <input
              type="number"
              inputmode="numeric"
              min="3"
              max="8"
              bind:value={ft}
            /></label
          >
          <label
            >(in) <input
              type="number"
              inputmode="numeric"
              min="0"
              max="11"
              bind:value={inch}
            /></label
          >
        </div>
      {/if}
      <label>
        Weight today ({weightUnit})
        <input type="number" inputmode="decimal" step="0.1" bind:value={weightIn} />
      </label>
    {:else if step === 'target'}
      <label>
        Goal weight ({weightUnit})
        <input type="number" inputmode="decimal" step="0.1" bind:value={goalIn} />
      </label>
      {#if a.goal_weight_kg && !canGoOn}
        <p class="text-sm text-amber-700 dark:text-amber-400">
          {a.goal === 'lose'
            ? 'To lose weight, pick a goal below your current weight.'
            : 'To gain weight, pick a goal above your current weight.'}
        </p>
      {/if}
      <p class="mt-2 text-sm font-medium">Pace</p>
      <div class="grid grid-cols-2 gap-2">
        {#each paces as p (p)}
          <button class="choice" class:chosen={a.pace_kg === p} onclick={() => (a.pace_kg = p)}>
            <span class="font-semibold">{formatPace(p, a.units)}</span>
            {#if canGoOn}<span class="text-sm text-zinc-500 dark:text-zinc-400"
                >about {roughDate(p)}</span
              >{/if}
          </button>
        {/each}
      </div>
      <p class="text-sm text-zinc-500 dark:text-zinc-400">
        Half a kilo a week is a steady pace most people can keep up.
      </p>
    {:else if step === 'activity'}
      {#each activities as act (act.id)}
        <button
          class="choice"
          class:chosen={a.activity === act.id}
          onclick={() => (a.activity = act.id)}
        >
          <span class="font-semibold">{act.label}</span>
          <span class="text-sm text-zinc-500 dark:text-zinc-400">{act.hint}</span>
        </button>
      {/each}
      <p class="text-sm text-zinc-500 dark:text-zinc-400">
        Don't count workouts here. You'll log those, and they add to your budget that day.
      </p>
    {:else if step === 'fasting'}
      {#each fastings as f (f.id)}
        <button class="choice" class:chosen={a.fasting === f.id} onclick={() => (a.fasting = f.id)}>
          <span class="font-semibold">{f.label}</span>
          <span class="text-sm text-zinc-500 dark:text-zinc-400">{f.hint}</span>
        </button>
      {/each}
      {#if a.fasting !== 'none'}
        <label class="mt-2">
          My eating window opens at
          <input type="time" bind:value={a.eating_start} />
        </label>
        <p class="text-sm text-zinc-500 dark:text-zinc-400">
          You'd eat from {a.eating_start} to {eatingEnd(a.eating_start, a.fasting)}.
        </p>
        <fieldset>
          <legend class="text-sm font-medium">On these days</legend>
          <div class="mt-1 grid grid-cols-7 gap-1">
            {#each weekdays as d (d.id)}
              <button
                type="button"
                class="chip px-0 py-1.5"
                class:chosen={!a.fasting_days.length || a.fasting_days.includes(d.id)}
                aria-pressed={!a.fasting_days.length || a.fasting_days.includes(d.id)}
                onclick={() => toggleDay(d.id)}>{d.label}</button
              >
            {/each}
          </div>
          <p class="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
            A day's fast starts when its eating window closes.
          </p>
        </fieldset>
      {/if}
    {:else if step === 'diet'}
      {#each diets as d (d.id)}
        <button class="choice" class:chosen={a.diet === d.id} onclick={() => (a.diet = d.id)}>
          <span class="font-semibold">{d.label}</span>
          <span class="text-sm text-zinc-500 dark:text-zinc-400">{d.hint}</span>
        </button>
      {/each}
    {:else if plan}
      <div class="card text-center">
        <p class="text-sm text-zinc-500 dark:text-zinc-400">Your daily budget</p>
        <p class="text-5xl font-bold tracking-tight text-brand-600 dark:text-brand-400">
          {plan.budget.toLocaleString()}
        </p>
        <p class="text-sm text-zinc-500 dark:text-zinc-400">calories</p>
        <div class="mt-4 grid grid-cols-3 gap-2 text-sm">
          <div><b class="block text-lg">{plan.carbs_g} g</b>carbs</div>
          <div><b class="block text-lg">{plan.protein_g} g</b>protein</div>
          <div><b class="block text-lg">{plan.fat_g} g</b>fat</div>
        </div>
      </div>
      <dl class="card grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
        {#if plan.goal_date}
          <dt class="text-zinc-500 dark:text-zinc-400">Goal</dt>
          <dd>
            {formatWeight(a.goal_weight_kg, a.units)} by <b>{goalDate(plan.goal_date)}</b>,
            {formatPace(plan.pace_kg, a.units)}
          </dd>
        {/if}
        <dt class="text-zinc-500 dark:text-zinc-400">You burn</dt>
        <dd>about {plan.tdee.toLocaleString()} calories a day</dd>
        <dt class="text-zinc-500 dark:text-zinc-400">BMI</dt>
        <dd>{plan.bmi}</dd>
        {#if a.fasting !== 'none'}
          <dt class="text-zinc-500 dark:text-zinc-400">Eating</dt>
          <dd>
            {a.eating_start} to {eatingEnd(a.eating_start, a.fasting)}, {fastingDaysLabel(
              a.fasting_days,
            )}
          </dd>
        {/if}
      </dl>
      {#if plan.floored}
        <p
          class="rounded-xl bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:bg-amber-500/10 dark:text-amber-200"
        >
          I kept your budget at a safe minimum, so you'll go a little slower than the pace you
          picked.
        </p>
      {/if}
      <p class="text-xs text-zinc-500 dark:text-zinc-400">
        These are estimates, not medical advice. Your budget adjusts as you log your weight.
      </p>
    {:else if planError}
      <p role="alert">{planError}</p>
      <button class="btn-secondary" onclick={back}>Go back</button>
    {/if}
  </section>

  <div class="mt-6">
    {#if saveError}<p class="mb-3 text-sm text-red-600 dark:text-red-400" role="alert">
        {saveError}
      </p>{/if}
    {#if step === 'plan'}
      <button class="btn-primary w-full" disabled={!plan || busy} onclick={finish}>
        {busy ? 'Saving…' : 'Start tracking'}
      </button>
    {:else}
      <button class="btn-primary w-full" disabled={!canGoOn} onclick={next}>
        {step === 'welcome'
          ? "Let's go"
          : step === 'motivation' && !a.motivations.length
            ? 'Skip'
            : 'Continue'}
      </button>
    {/if}
  </div>
</main>
