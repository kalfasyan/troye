# Troye: architecture and build plan

Troye is a private calorie and weight tracker. It runs as one
container on your own hardware (a NAS or the Pi), installs on a phone, and keeps
every photo and log at home. Nothing is sent to a cloud API.

## Photo calorie counting

Models are rough when asked for calories straight from a photo: in a 2026
benchmark of ten models on Nutrition5k the best calorie error was about 80 kcal a
meal, and small open models are worse, especially on packaged food. So the model
never invents the calories:

1. **Identify** the items on the plate (a vision model).
2. **Estimate portions** in grams or household units, asking for the portion that
   is visible, not a standard serving.
3. **Look up** each item in a nutrition database, so calories and macros are real.
4. **Confirm**: the person sees a draft and nudges portions before it is logged.
5. **Learn** from corrections: usual foods and portions are suggested first.

Packaged food is logged by barcode (Open Food Facts) or by a photo of the
nutrition label.

## What runs where

| Machine | Role |
|---|---|
| NAS or Pi 5 (16 GB) | The app: database, photos, web app, push, jobs. Always on. |
| GPU PC (RTX 3060 or RX 7900 XT) | The vision worker, with Ollama. Works through queued photos when it is on. |
| Pi 5 CPU | Optional fallback worker with a small vision model. Slow, off by default. |
| Hailo-8 | Can't run LLMs or VLMs. Later, maybe a fast food detector for instant drafts. |

A photo flows like this: the phone shrinks it and strips its location, uploads
it as a `meal_photos` record with status `queued`, the worker claims it, writes
a draft (items, grams, confidence, matched foods), and a Go hook sends a push.
Photos stay in `pb_data`, deduplicated by hash; originals are downscaled after a
retention period.

## Nutrition data

- USDA FoodData Central (public domain), bundled: generic foods with portions.
- Open Food Facts: packaged foods by barcode, cached after the first lookup.
- The person's own foods and recipes.

## Onboarding and budget

An interview on first sign-in: goal and target weight, sex, birth date, height,
weight, activity, pace (0.25 to 1 kg a week), fasting schedule, motivations,
macro preference. The budget is Mifflin-St Jeor times activity, minus the deficit
for the pace, never below 1,200 kcal (women) or 1,500 kcal (men), and it is
recalculated as weight changes.

## Build plan

Each step is one PR that leaves the app working.

1. Turn the template into Troye: rename and remove the demo (branding comes separately).
2. Profile and onboarding interview, with the budget calculation.
3. Food diary: USDA foods, search, recents, custom foods, barcode lookup.
4. Weight tracking and its charts (hand-drawn SVG, no chart library).
5. Photo logging: capture, storage, queue, the Ollama vision worker, confirm screen, push.
6. Accuracy loop: score models on confirmed photos, so the model is picked by numbers.
7. Extras: fasting timer, water, exercise, weekly summary.
