"""Running a worker on an interval."""

from __future__ import annotations

import logging
import time
from collections.abc import Callable

from .client import Client

log = logging.getLogger("plasto_worker")


def every(seconds: float, fn: Callable[[], None], client: Client, *, once: bool = False) -> None:
    """Calls fn every `seconds`, forever, sending a heartbeat each round.

    An exception in fn is logged and the loop carries on, so one bad round
    doesn't stop the worker. With once=True it runs a single round (handy in tests
    and cron).
    """
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    logging.getLogger("httpx").setLevel(logging.WARNING)  # not every request
    while True:
        started = time.monotonic()
        try:
            client.heartbeat()
            fn()
        except Exception:
            if once:
                raise
            log.exception("Round failed")
        if once:
            return
        time.sleep(max(0, seconds - (time.monotonic() - started)))
