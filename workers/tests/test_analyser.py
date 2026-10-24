import base64
import json

import httpx

import analyser
from plasto_worker import Client
from tests.conftest import fake_ollama, tiny_png


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
