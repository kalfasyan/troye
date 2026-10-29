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

The analyser on the GPU PC starts on boot and checks in with the NAS every
minute, so the app shows whether it is on and how many photos wait. When it
comes online it works through the queue and each person gets a push as their
meals are ready. When it is off, a Wake PC button has the NAS send a
Wake-on-LAN packet. Logging never waits on it: search and barcode work at once.

The app is for a household: each person has their own account, and their diary,
photos, weigh-ins and plan are visible only to them.

## Hosting

The app runs on a Synology DS220+ (x86-64, Container Manager), set up like Flowy:

- `deploy/install.sh` over SSH, on port 8092 next to Splitwiz (8090) and Flowy (8091).
- `https://troye.<name>.synology.me` through the DSM reverse proxy with the
  existing Let's Encrypt wildcard certificate, so both phones reach it at home and
  away with no VPN app, and push and app install work (they need HTTPS).
- Tailscale stays an option for keeping it off the internet.
- The analyser on the GPU PC talks to the NAS over the home network. The NAS
  sends the Wake-on-LAN packet as a broadcast on the home network.
- Photos stay in `pb_data` on the NAS, inside the nightly backups and Hyper Backup.

## Nutrition data

Generic foods in English first, with Belgian and Dutch extras:

- USDA FoodData Central (SR Legacy and Foundation Foods): about 8,000 generic foods
  with English names, in the public domain, so it ships with the app. (Ciqual was
  the first choice, but its site was down for weeks, so it was dropped.)
- NEVO (RIVM, Netherlands): about 2,300 Dutch and Flemish foods. It is free but
  can't be redistributed, so you download it once and Troye imports it, citing
  "NEVO-online version 2025/9.0, RIVM, Bilthoven".
- Open Food Facts: packaged foods by barcode, including supermarket own brands;
  Belgian and Dutch products are kept locally so lookups work offline.
- USDA FoodData Central (public domain): a broad fallback with standard portions.
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
3. Food diary: USDA and NEVO foods, search, recents, custom foods, barcode lookup.
4. Weight tracking and its charts (hand-drawn SVG, no chart library).
5. Photo logging: capture, storage, queue, the Ollama vision worker, confirm screen, push.
6. Accuracy loop: score models on confirmed photos, so the model is picked by numbers.
7. Extras: fasting timer, water, exercise, weekly summary.
8. Lose It! import: the food log, weigh-ins, exercise, own foods, meal photos and plan from an export, with the nutrients, fasting days and tracker calories it needed.
