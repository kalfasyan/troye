"""Starts a real troye server for the tests, built from ../backend."""

import json
import os
import socket
import struct
import subprocess
import time
import zlib
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
    tmp = tmp_path_factory.mktemp("troye")
    binary = os.environ.get("TROYE_BIN")
    if not binary:
        binary = str(tmp / "troye")
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


def tiny_png() -> bytes:
    def chunk(kind: bytes, data: bytes) -> bytes:
        body = kind + data
        return struct.pack(">I", len(data)) + body + struct.pack(">I", zlib.crc32(body))

    raw = b"\x00" + b"\x00\x00\x00" * 2
    return (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack(">IIBBBBB", 2, 1, 8, 2, 0, 0, 0))
        + chunk(b"IDAT", zlib.compress(raw))
        + chunk(b"IEND", b"")
    )


def fake_ollama(answer: dict, seen: list) -> httpx.Client:
    def handle(req: httpx.Request) -> httpx.Response:
        if req.url.path == "/api/tags":
            return httpx.Response(200, json={"models": []})
        body = json.loads(req.content)
        seen.append(body)
        return httpx.Response(200, json={"message": {"content": json.dumps(answer)}})

    return httpx.Client(base_url="http://ollama", transport=httpx.MockTransport(handle))


@pytest.fixture
def person(server, admin):
    """A user with a queued lunch photo; returns (user id, token, photo id)."""
    email = "ada@example.com"
    res = admin.get("/api/collections/users/records", params={"filter": f'email = "{email}"'})
    if not res.json()["items"]:
        admin.post(
            "/api/collections/users/records",
            json={"email": email, "password": "password123", "passwordConfirm": "password123"},
        ).raise_for_status()
        admin.post(
            "/api/collections/foods/records",
            json={"name": "Rice, white, cooked", "source": "usda", "kcal": 130, "protein": 2.7},
        ).raise_for_status()
    auth = httpx.post(
        f"{server}/api/collections/users/auth-with-password",
        json={"identity": email, "password": "password123"},
    ).json()
    token, uid = auth["token"], auth["record"]["id"]
    photo = httpx.post(
        f"{server}/api/collections/meal_photos/records",
        headers={"Authorization": token},
        data={"user": uid, "date": "2026-10-24", "meal": "lunch", "note": "no sauce"},
        files={"photo": ("lunch.png", tiny_png(), "image/png")},
    )
    photo.raise_for_status()
    return uid, token, photo.json()["id"]
