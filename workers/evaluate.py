"""Scores vision models on the photos you have already logged, so the model the
analyser uses is picked by numbers rather than by feel.

    TROYE_URL=... TROYE_WORKER=analyser TROYE_WORKER_PASSWORD=... \\
        uv run python evaluate.py qwen2.5vl:7b gemma3:12b llava:13b

Each model gets every logged photo (or the newest --limit of them), its items
are matched to foods the way the app does (without what it learned from these
same photos), and the draft is compared with what was logged. Nothing is
written; pull the models with `ollama pull` first. The web app's Me tab shows
the same scores for the drafts people actually saw (web/src/lib/accuracy.ts).
"""

from __future__ import annotations

import argparse
import os
import statistics
import sys
import time
from dataclasses import dataclass, field
from typing import Any

import httpx

from analyser import Analyser
from plasto_worker import Client


@dataclass
class PhotoScore:
    kcal: float | None  # the draft against what was logged, as a fraction: 0.1 is 10% over
    found: int  # foods logged that the draft had
    logged: int  # foods logged
    extra: int  # draft items that weren't kept
    drafted: int
    grams: list[float] = field(default_factory=list)  # portion misses on foods it had right


def score_photo(draft: list[dict[str, Any]], logged: list[dict[str, Any]]) -> PhotoScore:
    draft_kcal = sum(i.get("kcal", 0) for i in draft)
    logged_kcal = sum(i.get("kcal", 0) for i in logged)
    kept = {i["food"] for i in logged if i.get("food")}
    score = PhotoScore(
        kcal=(draft_kcal - logged_kcal) / logged_kcal if logged_kcal > 0 else None,
        found=0,
        logged=len([i for i in logged if i.get("food")]),
        extra=len([i for i in draft if not i.get("food") or i["food"] not in kept]),
        drafted=len(draft),
    )
    for item in logged:
        if not item.get("food"):
            continue
        hit = next((d for d in draft if d.get("food") == item["food"]), None)
        if hit is None:
            continue
        score.found += 1
        if item.get("grams", 0) > 0:
            score.grams.append(abs(hit["grams"] - item["grams"]) / item["grams"])
    return score


def summarise(model: str, scores: list[PhotoScore], seconds: float = 0) -> dict[str, Any]:
    kcal = [s.kcal for s in scores if s.kcal is not None]
    logged = sum(s.logged for s in scores)
    drafted = sum(s.drafted for s in scores)
    grams = [g for s in scores for g in s.grams]
    return {
        "model": model,
        "photos": len(scores),
        "kcal_off": statistics.median(abs(k) for k in kcal) if kcal else None,
        "kcal_bias": statistics.mean(kcal) if kcal else None,
        "found": sum(s.found for s in scores) / logged if logged else None,
        "extra": sum(s.extra for s in scores) / drafted if drafted else None,
        "grams_off": statistics.median(grams) if grams else None,
        "seconds": seconds,
    }


def table(rows: list[dict[str, Any]]) -> str:
    def pct(x: float | None, signed: bool = False) -> str:
        if x is None:
            return "–"
        return f"{x * 100:+.0f}%" if signed else f"{abs(x) * 100:.0f}%"

    lines = [
        "| Model | Photos | Calories off | Over/under | Foods found | Extra items "
        "| Portions off | Seconds a photo |",
        "|---|---|---|---|---|---|---|---|",
    ]
    for r in rows:
        secs = f"{r['seconds']:.1f}" if r["seconds"] else "–"
        lines.append(
            f"| {r['model']} | {r['photos']} | {pct(r['kcal_off'])} | "
            f"{pct(r['kcal_bias'], signed=True)} | {pct(r['found'])} | {pct(r['extra'])} | "
            f"{pct(r['grams_off'])} | {secs} |"
        )
    return "\n".join(lines)


def logged_photos(app: Client, limit: int | None) -> list[dict[str, Any]]:
    photos = app.list("meal_photos", filter='status = "logged"', sort="-created")
    photos = [p for p in photos if p.get("confirmed")]
    return photos[:limit] if limit else photos


def evaluate(app: Client, ollama: httpx.Client, models: list[str], limit: int | None = None):
    photos = logged_photos(app, limit)
    # how the drafts people saw did, per model that made them
    rows = []
    for model in sorted({p.get("model") or "unknown" for p in photos}):
        mine = [p for p in photos if (p.get("model") or "unknown") == model and p.get("items")]
        scores = [score_photo(p["items"], p["confirmed"]) for p in mine]
        rows.append(summarise(f"{model} (as used)", scores))

    token = app.request("POST", "/api/files/token")["token"]
    for model in models:
        analyser = Analyser(app, ollama, model)
        scores, started = [], time.monotonic()
        for p in photos:
            image = app.download(f"{app.file_url(p, p['photo'])}?token={token}")
            seen = analyser.analyse(image, p["meal"], p.get("note", ""))
            draft = app.request(
                "POST", "/api/app/photos/match", json={"user": p["user"], "items": seen}
            )["items"]
            scores.append(score_photo(draft, p["confirmed"]))
        each = (time.monotonic() - started) / len(photos) if photos else 0
        rows.append(summarise(model, scores, each))
    return rows


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("models", nargs="*", help="Ollama models to try, such as qwen2.5vl:7b")
    parser.add_argument("--limit", type=int, help="only the newest N logged photos")
    args = parser.parse_args(argv)

    app = Client()
    url = os.environ.get("OLLAMA_URL", "http://127.0.0.1:11434")
    with httpx.Client(base_url=url) as ollama:
        if args.models and not Analyser(app, ollama).ollama_ready():
            sys.exit(f"Ollama isn't answering at {url}. Start it, or set OLLAMA_URL.")
        rows = evaluate(app, ollama, args.models, args.limit)
    if not rows:
        sys.exit("No logged photos yet. Log a few meals from photos first.")
    print(table(rows))


if __name__ == "__main__":
    main()
