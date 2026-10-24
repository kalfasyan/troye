"""Starts a real plasto server for the tests, built from ../backend."""

import os
import socket
import subprocess
import time
from pathlib import Path

import httpx
import pytest

ROOT = Path(__file__).resolve().parents[2]


def _free_port() -> int:
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


@pytest.fixture(scope="session")
def server(tmp_path_factory):
    tmp = tmp_path_factory.mktemp("plasto")
    binary = os.environ.get("PLASTO_BIN")
    if not binary:
        binary = str(tmp / "plasto")
        subprocess.run(["go", "build", "-o", binary, "."], cwd=ROOT / "backend", check=True)

    data, port = tmp / "pb_data", _free_port()
    url = f"http://127.0.0.1:{port}"
    subprocess.run(
        [binary, "superuser", "upsert", "admin@example.com", "password123", f"--dir={data}"],
        check=True,
        capture_output=True,
    )
    proc = subprocess.Popen(
        [binary, "serve", f"--http=127.0.0.1:{port}", f"--dir={data}", f"--publicDir={tmp}"],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    try:
        for _ in range(100):
            try:
                if httpx.get(f"{url}/api/health").status_code == 200:
                    break
            except httpx.TransportError:
                time.sleep(0.1)
        yield url
    finally:
        proc.terminate()
        proc.wait()


@pytest.fixture(scope="session")
def admin(server):
    """An HTTP client signed in as a superuser, for setting up test data."""
    res = httpx.post(
        f"{server}/api/collections/_superusers/auth-with-password",
        json={"identity": "admin@example.com", "password": "password123"},
    )
    res.raise_for_status()
    with httpx.Client(base_url=server, headers={"Authorization": res.json()["token"]}) as c:
        # a collection of the tests' own, so they don't depend on the app's
        workers = '@request.auth.collectionName = "workers"'
        c.post(
            "/api/collections",
            json={
                "name": "notes",
                "fields": [{"name": "title", "type": "text", "required": True}],
                **{f"{r}Rule": workers for r in ("list", "view", "create", "update", "delete")},
            },
        ).raise_for_status()
        yield c


@pytest.fixture
def worker(server, admin, request):
    """A fresh worker account; returns (name, password)."""
    name = request.node.name.lower().replace("_", "-")[:50]
    password = "worker-password-1"
    res = admin.post(
        "/api/collections/workers/records",
        json={"name": name, "password": password, "passwordConfirm": password},
    )
    res.raise_for_status()
    return name, password
