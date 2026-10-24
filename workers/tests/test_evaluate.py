import httpx

import analyser
import evaluate
from plasto_worker import Client
from tests.conftest import fake_ollama


def test_score_photo():
    rice = {"food": "rice", "grams": 180, "kcal": 234}
    chicken = {"food": "chicken", "grams": 130, "kcal": 215}
    s = evaluate.score_photo(
        [rice, chicken, {"food": "", "grams": 40, "kcal": 0}],
        [{**rice, "grams": 150, "kcal": 195}, chicken, {"food": "", "grams": 0, "kcal": 80}],
    )
    assert round(s.kcal, 4) == round(-41 / 490, 4)
    assert (s.found, s.logged, s.extra, s.drafted) == (2, 2, 1, 3)
    assert s.grams == [0.2, 0]


def test_it_scores_models_on_logged_photos(server, admin, worker, person):
    uid, token, photo_id = person
    rice = admin.get(
        "/api/collections/foods/records", params={"filter": 'name = "Rice, white, cooked"'}
    ).json()["items"][0]["id"]

    # the analyser drafts 180 g of rice; 150 g gets logged
    first = {"items": [{"name": "white rice", "grams": 180, "confidence": 0.8}]}
    with Client(server, *worker) as app, fake_ollama(first, []) as ollama:
        analyser.Analyser(app, ollama).drain()
    httpx.post(
        f"{server}/api/app/photos/{photo_id}/log",
        headers={"Authorization": token},
        json={"items": [{"name": "white rice", "food": rice, "grams": 150}]},
    ).raise_for_status()

    # a better model says 150 g
    better = {"items": [{"name": "rice", "grams": 150, "confidence": 0.9}]}
    with Client(server, *worker) as app, fake_ollama(better, []) as ollama:
        rows = evaluate.evaluate(app, ollama, ["better:7b"])

    used = next(r for r in rows if r["model"].endswith("(as used)"))
    assert used["kcal_off"] > 0.15 and used["found"] == 1
    tried = next(r for r in rows if r["model"] == "better:7b")
    assert tried["photos"] >= 1
    assert tried["kcal_off"] == 0
    assert tried["grams_off"] == 0
    assert "| better:7b |" in evaluate.table(rows)
