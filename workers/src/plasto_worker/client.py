"""A small client for the plasto API, signed in as a worker account."""

from __future__ import annotations

import json
import logging
import os
import time
from collections.abc import Callable, Iterator
from typing import Any

import httpx

log = logging.getLogger("plasto_worker")

Record = dict[str, Any]


class ApiError(Exception):
    """The server answered with an error (a 4xx or 5xx status)."""

    def __init__(self, status: int, message: str, data: Any = None):
        super().__init__(f"{status}: {message}")
        self.status = status
        self.message = message
        self.data = data


class Client:
    """Talks to a plasto app as a worker.

    Settings come from the arguments or from the PLASTO_URL, PLASTO_WORKER and
    PLASTO_WORKER_PASSWORD environment variables. The client signs in on first
    use, signs in again when its token expires, and retries while the server is
    unreachable (for example during an update).
    """

    def __init__(
        self,
        url: str | None = None,
        name: str | None = None,
        password: str | None = None,
        *,
        retries: int = 5,
        timeout: float = 30,
    ):
        env = os.environ.get
        self.url = (url or env("PLASTO_URL") or "http://127.0.0.1:8090").rstrip("/")
        self.name = name or env("PLASTO_WORKER", "")
        self.password = password or env("PLASTO_WORKER_PASSWORD", "")
        if not self.name or not self.password:
            raise ValueError(
                "Set PLASTO_WORKER and PLASTO_WORKER_PASSWORD to a worker account's name "
                "and password."
            )
        self.retries = retries
        self._http = httpx.Client(base_url=self.url, timeout=timeout)
        self._token = ""

    # --- records ---------------------------------------------------------------

    def list(self, collection: str, *, filter: str = "", sort: str = "") -> list[Record]:
        """All records of a collection that match the filter, e.g. filter="done = false"."""
        items: list[Record] = []
        page = 1
        while True:
            params = {"page": page, "perPage": 200, "skipTotal": 1}
            if filter:
                params["filter"] = filter
            if sort:
                params["sort"] = sort
            res = self.request("GET", f"/api/collections/{collection}/records", params=params)
            items.extend(res["items"])
            if len(res["items"]) < 200:
                return items
            page += 1

    def get(self, collection: str, id: str) -> Record:
        return self.request("GET", f"/api/collections/{collection}/records/{id}")

    def create(self, collection: str, data: Record, files: dict[str, Any] | None = None) -> Record:
        """Adds a record. `files` maps file fields to (filename, bytes or file) tuples."""
        return self._write("POST", f"/api/collections/{collection}/records", data, files)

    def update(
        self, collection: str, id: str, data: Record, files: dict[str, Any] | None = None
    ) -> Record:
        return self._write("PATCH", f"/api/collections/{collection}/records/{id}", data, files)

    def delete(self, collection: str, id: str) -> None:
        self.request("DELETE", f"/api/collections/{collection}/records/{id}")

    def file_url(self, record: Record, filename: str) -> str:
        """Where to download a file stored in a record's file field."""
        return f"{self.url}/api/files/{record['collectionId']}/{record['id']}/{filename}"

    def heartbeat(self) -> None:
        """Tells the app this worker is alive; shown as last_seen in the admin UI."""
        self.request("POST", "/api/app/workers/heartbeat")

    # --- realtime ----------------------------------------------------------------

    def subscribe(self, topic: str, handler: Callable[[str, Record], None]) -> None:
        """Calls handler(action, record) for every change to a collection, forever.

        `topic` is a collection name ("items") or one record ("items/<id>"). The
        action is "create", "update" or "delete". Reconnects when the connection drops.
        """
        delay = 1.0
        while True:
            try:
                for event, data in self._events(topic):
                    delay = 1.0
                    if event != topic:
                        continue
                    try:
                        handler(data["action"], data["record"])
                    except Exception:
                        log.exception("Handling a change to %s failed", topic)
            except (httpx.TransportError, ApiError) as err:
                log.warning("Realtime connection lost (%s); reconnecting in %.0fs", err, delay)
            time.sleep(delay)
            delay = min(delay * 2, 60)

    def _events(self, topic: str) -> Iterator[tuple[str, Record]]:
        with self._http.stream("GET", "/api/realtime", timeout=None) as res:
            if res.status_code >= 400:
                raise ApiError(res.status_code, "could not open the realtime connection")
            for event, data in _sse(res.iter_lines()):
                if event == "PB_CONNECT":
                    self.request(
                        "POST",
                        "/api/realtime",
                        json={"clientId": data["clientId"], "subscriptions": [topic]},
                    )
                    log.info("Listening for changes to %s", topic)
                else:
                    yield event, data

    # --- plumbing ----------------------------------------------------------------

    def login(self) -> None:
        res = self._http.post(
            "/api/collections/workers/auth-with-password",
            json={"identity": self.name, "password": self.password},
        )
        if res.status_code >= 400:
            raise ApiError(res.status_code, f"worker {self.name!r} could not sign in", _body(res))
        self._token = res.json()["token"]
        log.info("Signed in as worker %s", self.name)

    def request(self, method: str, path: str, **kwargs: Any) -> Any:
        """Sends an API request and returns the JSON answer (None for an empty one)."""
        attempt = 0
        while True:
            try:
                if not self._token:
                    self.login()
                res = self._http.request(
                    method, path, headers={"Authorization": self._token}, **kwargs
                )
                if res.status_code == 401:
                    # the token expired or the password changed: sign in once more
                    self._token = ""
                    self.login()
                    res = self._http.request(
                        method, path, headers={"Authorization": self._token}, **kwargs
                    )
            except httpx.TransportError as err:
                attempt += 1
                if attempt > self.retries:
                    raise
                wait = min(2**attempt, 60)
                log.warning("Server unreachable (%s); retrying in %ss", err, wait)
                time.sleep(wait)
                continue

            if res.status_code >= 400:
                body = _body(res)
                message = body.get("message", res.reason_phrase) if isinstance(body, dict) else ""
                raise ApiError(res.status_code, message, body)
            return res.json() if res.content else None

    def _write(self, method: str, path: str, data: Record, files: dict[str, Any] | None) -> Any:
        if not files:
            return self.request(method, path, json=data)
        form = {k: v if isinstance(v, str) else json.dumps(v) for k, v in data.items()}
        return self.request(method, path, data=form, files=files)

    def close(self) -> None:
        self._http.close()

    def __enter__(self) -> Client:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()


def _body(res: httpx.Response) -> Any:
    try:
        return res.json()
    except ValueError:
        return res.text


def _sse(lines: Iterator[str]) -> Iterator[tuple[str, Any]]:
    """Parses a server-sent event stream into (event, data) pairs."""
    event, data = "message", []
    for line in lines:
        if line == "":
            if data:
                yield event, json.loads("\n".join(data))
            event, data = "message", []
        elif line.startswith("event:"):
            event = line[6:].strip()
        elif line.startswith("data:"):
            data.append(line[5:].lstrip())
