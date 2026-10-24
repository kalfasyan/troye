"""DEMO: a worker that adds an item to the list every few minutes.

Everyone else gets a push notification for each one (backend/app/demo.go).
Run it with:

    PLASTO_WORKER=demo PLASTO_WORKER_PASSWORD=... uv run python demo.py

scripts/strip-demo.sh removes it.
"""

import sys
from datetime import datetime

from plasto_worker import Client, every


def main(once: bool = False) -> None:
    app = Client()

    def add_item() -> None:
        app.create("items", {"title": f"Hello from the demo worker at {datetime.now():%H:%M}"})

    every(5 * 60, add_item, app, once=once)


if __name__ == "__main__":
    main(once="--once" in sys.argv)
