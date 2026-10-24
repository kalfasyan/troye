import base64
import json
import struct
import zlib

import httpx
import pytest

import analyser
from plasto_worker import Client


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
            json={"name": "Rice, white, cooked", "source": "ciqual", "kcal": 130, "protein": 2.7},
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


def test_it_analyses_queued_photos(server, worker, person):
    _, token, photo_id = person
    seen: list = []
    answer = {"items": [{"name": "white rice", "grams": 180.4, "confidence": 0.8}]}
    with Client(server, *worker) as app, fake_ollama(answer, seen) as ollama:
        assert analyser.Analyser(app, ollama).drain() >= 1

    # the photo went to the model with the meal and the note
    assert base64.b64decode(seen[-1]["messages"][0]["images"][0]) == tiny_png()
    assert "lunch" in seen[-1]["messages"][0]["content"]
    assert "no sauce" in seen[-1]["messages"][0]["content"]

    photo = httpx.get(
        f"{server}/api/collections/meal_photos/records/{photo_id}",
        headers={"Authorization": token},
    ).json()
    assert photo["status"] == "ready"
    assert photo["model"] == analyser.MODEL
    item = photo["items"][0]
    assert item["food_name"] == "Rice, white, cooked"
    assert item["grams"] == 180
    assert item["kcal"] == 234


def test_an_empty_answer_fails_the_photo(server, worker, person):
    _, token, photo_id = person
    with Client(server, *worker) as app, fake_ollama({"items": []}, []) as ollama:
        analyser.Analyser(app, ollama).drain()
    photo = httpx.get(
        f"{server}/api/collections/meal_photos/records/{photo_id}",
        headers={"Authorization": token},
    ).json()
    assert photo["status"] == "failed"
    assert photo["error"]


def test_photos_wait_while_ollama_is_off(server, worker, person):
    _, token, photo_id = person

    def down(req: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("refused")

    with (
        Client(server, *worker) as app,
        httpx.Client(base_url="http://ollama", transport=httpx.MockTransport(down)) as ollama,
    ):
        assert analyser.Analyser(app, ollama).drain() == 0
    photo = httpx.get(
        f"{server}/api/collections/meal_photos/records/{photo_id}",
        headers={"Authorization": token},
    ).json()
    assert photo["status"] == "queued"


def test_parse_cleans_the_answer():
    content = json.dumps(
        {
            "items": [
                {"name": " Fries ", "grams": "120", "confidence": 0.6},
                {"name": "", "grams": 50, "confidence": 1},
                {"name": "plate", "grams": 0, "confidence": 1},
                {"name": "soup", "grams": "lots", "confidence": 1},
            ]
        }
    )
    assert analyser.parse(content) == [{"name": "Fries", "grams": 120, "confidence": 0.6}]
