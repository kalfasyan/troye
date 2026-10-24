"""The photo analyser: names the food in queued meal photos and estimates grams.

Runs on the GPU PC next to Ollama, and starts when the PC does:

    TROYE_URL=https://troye.example.com TROYE_WORKER=analyser TROYE_WORKER_PASSWORD=... \\
        uv run python analyser.py

It asks the app for a photo every 20 seconds (which is also how the app knows
the PC is on), sends it to a vision model in Ollama, and posts back the items
with their grams. The app matches them to foods, so the calories come from the
food tables and never from the model. While Ollama isn't answering, photos
stay in the queue.

Settings: OLLAMA_URL (http://127.0.0.1:11434) and TROYE_MODEL (qwen2.5vl:7b).
"""

from __future__ import annotations

import base64
import json
import logging
import os
import sys
from typing import Any

import httpx

from plasto_worker import Client, every

log = logging.getLogger("analyser")

MODEL = "qwen2.5vl:7b"

# what the model has to answer with; Ollama holds it to this shape
SCHEMA = {
    "type": "object",
    "properties": {
        "items": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "name": {"type": "string"},
                    "grams": {"type": "number"},
                    "confidence": {"type": "number"},
                },
                "required": ["name", "grams", "confidence"],
            },
        }
    },
    "required": ["items"],
}

PROMPT = """This is a photo of someone's {meal}. List each separate food or drink in it.

For each one give:
- name: a short, plain English name as a nutrition table would have it, with how
  it is cooked when you can tell, such as "white rice, cooked", "chicken breast,
  roasted", "mixed green salad", "french fries", "espresso". Name a dish's parts
  separately when they are clearly separate on the plate.
- grams: the weight of the portion you can see, not a standard serving. Use the
  plate (a dinner plate is about 26 cm across), bowls, cutlery and hands for scale.
- confidence: from 0 to 1, how sure you are of the name and the amount together.

Leave out plates, cutlery, packaging and anything that isn't eaten.{note}"""


class Analyser:
    def __init__(self, app: Client, ollama: httpx.Client, model: str = MODEL):
        self.app = app
        self.ollama = ollama
        self.model = model

    def ollama_ready(self) -> bool:
        try:
            return self.ollama.get("/api/tags", timeout=5).status_code == 200
        except httpx.TransportError:
            return False

    def drain(self) -> int:
        """Analyses photos until none are waiting; returns how many it did."""
        if not self.ollama_ready():
            log.warning("Ollama isn't answering at %s; leaving photos queued", self.ollama.base_url)
            return 0
        done = 0
        while job := self.app.request("POST", "/api/app/photos/claim"):
            self.handle(job)
            done += 1
        return done

    def handle(self, job: dict[str, Any]) -> None:
        log.info("Analysing photo %s", job["id"])
        try:
            image = self.app.download(job["url"])
            items = self.analyse(image, job.get("meal", "meal"), job.get("note", ""))
            draft: dict[str, Any] = {"model": self.model, "items": items}
            if not items:
                draft["error"] = "No food found in the photo."
        except Exception as err:
            log.exception("Photo %s failed", job["id"])
            draft = {"model": self.model, "error": f"The analyser couldn't read it ({err})."[:500]}
        self.app.request("POST", f"/api/app/photos/{job['id']}/draft", json=draft)

    def analyse(self, image: bytes, meal: str, note: str = "") -> list[dict[str, Any]]:
        hint = f'\n\nThe person added: "{note}"' if note else ""
        res = self.ollama.post(
            "/api/chat",
            json={
                "model": self.model,
                "stream": False,
                "format": SCHEMA,
                "keep_alive": "15m",
                "options": {"temperature": 0},
                "messages": [
                    {
                        "role": "user",
                        "content": PROMPT.format(meal=meal, note=hint),
                        "images": [base64.b64encode(image).decode()],
                    }
                ],
            },
            timeout=300,
        )
        res.raise_for_status()
        return parse(res.json()["message"]["content"])


def parse(content: str) -> list[dict[str, Any]]:
    """The items from the model's answer, cleaned up."""
    data = json.loads(content)
    items = []
    for it in data.get("items", []):
        name = str(it.get("name", "")).strip()
        try:
            grams = float(it.get("grams", 0))
            confidence = float(it.get("confidence", 0.5))
        except (TypeError, ValueError):
            continue
        if name and grams > 0:
            items.append({"name": name, "grams": round(grams), "confidence": confidence})
    return items


def main(once: bool = False) -> None:
    app = Client()
    url = os.environ.get("OLLAMA_URL", "http://127.0.0.1:11434")
    with httpx.Client(base_url=url) as ollama:
        analyser = Analyser(app, ollama, os.environ.get("TROYE_MODEL", MODEL))
        log.info("Analysing photos with %s at %s", analyser.model, url)
        every(20, analyser.drain, app, once=once)


if __name__ == "__main__":
    main(once="--once" in sys.argv)
