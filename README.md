<div align="center">

<img src="web/public/icons/icon.svg" width="88" alt="">

# Troye

**A private calorie and weight tracker that runs on your own hardware.**

</div>

Troye is a self-hosted calorie counter: an onboarding interview sets your daily
calorie budget from your goal, you log food by search, barcode or photo, and you
follow your weight on a trend chart. Photos are analysed by a vision model on
your own GPU PC, never by a cloud service, and every photo and log stays at home.

<table>
  <tr>
    <td align="center" width="33%"><img src="docs/images/screens/diary.png" alt="Today's diary: a ring of calories left, coloured by carbs, protein and fat, above the fasting window and the meals"><br><sub><b>Your day at a glance.</b> Calories left, macros and your eating window.</sub></td>
    <td align="center" width="33%"><img src="docs/images/screens/add-food.png" alt="Adding hummus by the tablespoon, with its calories and macros"><br><sub><b>Log in real amounts.</b> Tablespoons, slices and glasses, not only grams.</sub></td>
    <td align="center" width="33%"><img src="docs/images/screens/weight.png" alt="Weight chart with daily weigh-ins, a smoothed trend line and the goal"><br><sub><b>Watch the trend, not the noise.</b> A smoothed line and your goal date.</sub></td>
  </tr>
  <tr>
    <td align="center"><img src="docs/images/screens/week.png" alt="The week: calories per day against the budget, days logged and the streak"><br><sub><b>See your week.</b> Every day against its budget, and your streak.</sub></td>
    <td align="center"><img src="docs/images/screens/onboarding.png" alt="Plato, the plate mascot, welcoming a new person to the onboarding interview"><br><sub><b>A two-minute interview.</b> Plato sets your budget, macros and goal date.</sub></td>
    <td align="center"><img src="docs/images/screens/diary-dark.png" alt="Today's diary in dark mode"><br><sub><b>Light and dark.</b> An installable app that follows your phone.</sub></td>
  </tr>
</table>

It is a work in progress; [docs/plan.md](docs/plan.md) has the architecture and
the build plan. Troye is built on [plasto](https://github.com/selfhostit/plasto).

## Quick start

You need [Go](https://go.dev/dl/) 1.27+, [Node](https://nodejs.org) 22+ and
[uv](https://docs.astral.sh/uv/) (for the Python workers).

```sh
make setup   # install dependencies
make dev     # backend on :8090, web app with live reload on http://localhost:5173
```

Open the app and create your account: the first person becomes the **owner**
and can also sign in to the admin UI at http://localhost:8090/_/ with the same
email and password. Add more people there, in the `users` collection.

## What you get

- **First-run setup and sign-in.** No terminal needed to create the first account.
- **Users and roles.** The owner manages everyone; members can see each other and
  edit themselves. Nobody can sign themselves up.
- **Push notifications** straight from your server, with no account or third-party
  service. Each person turns them on per device; send one from Go with
  `push.ToUser(app, userID, push.Message{Title: "…", URL: "/…"})`. Phones need the
  app on HTTPS.
- **Scheduled jobs that don't get skipped.** `jobs.MustAdd(app, "digest", "0 7 * * *", fn)`
  runs on a cron schedule in your time zone (`TZ`). If the server was off when a job
  was due, it runs once when the server starts again. Every run and its error are
  listed in the admin UI under the `jobs` collection.
- **Nightly backups** of the database and uploads at 03:00, keeping the last 7, in
  `pb_data/backups`. Change the schedule or add S3 storage in the admin UI under
  Settings > Backups.
- **Python workers** for the heavy lifting (ML, scraping, LLMs), on this machine or
  another. They sign in with a worker account and read and write records through
  the API; see [workers/README.md](workers/README.md).
- **An installable app.** It installs to a phone's home screen or a desktop, opens
  offline from its cached shell, and follows the system's light or dark mode.

## How it fits together

```mermaid
flowchart LR
  phone["Phone or laptop<br/>installable web app"]
  subgraph box["Your NAS, Pi or server"]
    app["Go app on PocketBase<br/>API, auth, realtime, admin UI"]
    db[("SQLite and files<br/>pb_data/")]
    jobs["Scheduled jobs<br/>and nightly backups"]
    app --- db
    app --- jobs
  end
  worker["Python workers<br/>ML, scraping, LLMs"]
  phone -- "HTTPS over Tailscale" --> app
  worker -- "records through the API" --> app
  app -. "Web Push" .-> phone
```

Everything a person sees is the web app, served by the Go app. Heavy work runs in
Python workers, here or on another machine, which write records; Go hooks react to
those records and notify people.

## Deploy

On any Linux machine with Docker (a NAS, a Raspberry Pi, a server):

```sh
curl -fsSL https://raw.githubusercontent.com/selfhostit/troye/main/deploy/install.sh | sh
```

It asks for a Tailscale auth key, so your devices reach the app at
`https://troye.<your-tailnet>.ts.net`, and starts it with Docker Compose. See
[docs/deploy.md](docs/deploy.md) for updates, backups, workers, Synology and the Pi,
and [docs/synology.md](docs/synology.md) for the whole household setup on a Synology NAS.

## Layout

```
backend/            Go app on PocketBase
  main.go           entry point
  app/              routes, hooks and jobs (start here)
  jobs/             scheduled jobs with catch-up
  push/             Web Push: keys, subscriptions, sending
  migrations/       database schema as code
web/                Svelte 5 + Tailwind web app
  src/App.svelte    picks setup, sign-in or the app
  src/components/   screens; Home.svelte is where your app starts
  src/lib/          shared code (API client, session, install) and its tests
  public/           manifest, service worker and icons
workers/            Python worker kit, your workers, their tests and image
deploy/             compose files, Tailscale sidecar, install script, smoke test
docs/               the plan, the deploy guide and the first-feature guide
Dockerfile          the app's multi-arch image
Makefile            run `make` to see every command
```

## Commands

| Command      | What it does                                          |
| ------------ | ----------------------------------------------------- |
| `make dev`   | Run everything with live reload                       |
| `make check` | Format, lint and type-check                           |
| `make test`  | Run the Go, web and worker tests                      |
| `make build` | Build `dist/troye` with the web app next to it       |
| `make run`   | Build the Docker image and run it on :8090            |
| `make smoke` | Deploy both images with Compose and test end to end   |

Pushes to `main` publish `ghcr.io/selfhostit/troye` and
`ghcr.io/selfhostit/troye-worker` for amd64, arm64 and armv7; a `v*` tag
publishes that version.

## Support

If troye saves you time, you can [buy me a coffee](https://buymeacoffee.com/kalfasyan).

## License

MIT
