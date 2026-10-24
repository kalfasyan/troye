# plasto workers

Workers are Python programs that do the heavy lifting for a plasto app: object
detection, scraping, calling an LLM, crunching data. They run next to the app or
on another machine (a Raspberry Pi with a camera, say), sign in with a worker
account, and read and write records through the app's API. The app reacts to
those records: a Go hook sees a new record and sends a push notification, so
workers never deal with users or notifications themselves.

## 1. Create a worker account

In the admin UI (`/_/`), open the **workers** collection and add a record with a
`name` (like `camera`) and a password. To rotate the password, change it there
and restart the worker. To revoke a worker, delete its record.

## 2. Write the worker

Start from `worker.py`, or add a file next to it:

```python
from plasto_worker import Client, every

app = Client()  # reads PLASTO_URL, PLASTO_WORKER and PLASTO_WORKER_PASSWORD


def check():
    for item in app.list("items", filter='title ~ "milk"'):
        app.update("items", item["id"], {"title": item["title"].upper()})


every(60, check, app)  # forever, with a heartbeat each round
```

What the client gives you:

| Call | What it does |
| --- | --- |
| `app.list(collection, filter=, sort=)` | All matching records (paged for you) |
| `app.get` / `create` / `update` / `delete` | One record; `files={"photo": ("a.jpg", data)}` uploads |
| `app.subscribe("items", handler)` | Calls `handler(action, record)` on every change, reconnecting as needed |
| `app.heartbeat()` | Marks the worker as seen (`last_seen` in the admin UI) |
| `every(seconds, fn, app)` | Runs `fn` on an interval; a failing round is logged, not fatal |

The client signs in on first use, signs in again when its token expires, and
retries while the server is unreachable. API errors raise `ApiError` with the
server's message. Filters use [PocketBase's syntax](https://pocketbase.io/docs/api-rules-and-filters/).

What a worker may do is set by each collection's API rules: allow workers with
`@request.auth.collectionName = "workers"`.

## 3. Run it

You need [uv](https://docs.astral.sh/uv/).

```sh
cd workers
PLASTO_URL=http://localhost:8090 PLASTO_WORKER=... PLASTO_WORKER_PASSWORD=... uv run python worker.py
```

Add `--once` for a single round.

<!-- DEMO start -->
`demo.py` adds an item to the demo list every five minutes; the app shows it
live and sends everyone else a push notification.
<!-- DEMO end -->

## Tests

`uv run pytest` builds the app from `../backend`, starts it on a free port and
runs the client against it.
