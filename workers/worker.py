"""Your worker: put the work in run() and it runs every minute, with a heartbeat.

    PLASTO_WORKER=... PLASTO_WORKER_PASSWORD=... uv run python worker.py

See README.md for what the client can do, and demo.py for a worker that writes records.
"""

import sys

from plasto_worker import Client, every


def main(once: bool = False) -> None:
    app = Client()

    def run() -> None:
        pass  # e.g. for item in app.list("items", filter="done = false"): ...

    every(60, run, app, once=once)


if __name__ == "__main__":
    main(once="--once" in sys.argv)
