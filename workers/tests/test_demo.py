"""DEMO: tests for demo.py. scripts/strip-demo.sh removes them."""

import demo
from plasto_worker import Client


def test_demo_adds_an_item(server, worker, monkeypatch):
    name, password = worker
    monkeypatch.setenv("PLASTO_URL", server)
    monkeypatch.setenv("PLASTO_WORKER", name)
    monkeypatch.setenv("PLASTO_WORKER_PASSWORD", password)

    demo.main(once=True)

    with Client(server, name, password) as app:
        items = app.list("items", filter=f'by = "{name}"')
    assert len(items) == 1
    assert items[0]["title"].startswith("Hello from the demo worker")
