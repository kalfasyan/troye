import threading

import pytest

from plasto_worker import ApiError, Client
from plasto_worker.client import _sse


def test_records(server, worker):
    name, password = worker
    with Client(server, name, password) as app:
        item = app.create("notes", {"title": "From a test"})
        assert app.get("notes", item["id"])["title"] == "From a test"
        assert any(i["id"] == item["id"] for i in app.list("notes", filter='title ~ "test"'))


def test_errors_carry_the_server_message(server, worker):
    with Client(server, *worker) as app, pytest.raises(ApiError) as err:
        app.create("notes", {})
    assert err.value.status == 400


def test_heartbeat_sets_last_seen(server, admin, worker):
    name, password = worker
    with Client(server, name, password) as app:
        app.heartbeat()
    w = admin.get("/api/collections/workers/records", params={"filter": f'name = "{name}"'})
    assert w.json()["items"][0]["last_seen"]


def test_a_rotated_password_signs_in_again(server, admin, worker):
    name, password = worker
    app = Client(server, name, password)
    app.list("notes")
    wid = admin.get("/api/collections/workers/records", params={"filter": f'name = "{name}"'})
    wid = wid.json()["items"][0]["id"]
    new = "rotated-password-2"
    admin.patch(
        f"/api/collections/workers/records/{wid}", json={"password": new, "passwordConfirm": new}
    ).raise_for_status()

    app.password = new  # the worker restarted with the new password
    app.list("notes")


def test_a_revoked_worker_is_refused(server, admin, worker):
    name, password = worker
    wid = admin.get("/api/collections/workers/records", params={"filter": f'name = "{name}"'})
    admin.delete(f"/api/collections/workers/records/{wid.json()['items'][0]['id']}")
    with Client(server, name, password) as app, pytest.raises(ApiError) as err:
        app.list("notes")
    assert err.value.status == 400


def test_subscribe_sees_new_records(server, worker):
    seen = []
    got = threading.Event()

    def handler(action, record):
        seen.append((action, record["title"]))
        got.set()

    listener = Client(server, *worker)
    threading.Thread(target=listener.subscribe, args=("notes", handler), daemon=True).start()

    with Client(server, *worker) as app:
        # keep adding until the listener is connected and sees one
        for i in range(50):
            app.create("notes", {"title": f"live {i}"})
            if got.wait(0.2):
                break
    assert seen and seen[0][0] == "create" and seen[0][1].startswith("live")


def test_sse_parser():
    lines = [
        "id:abc",
        "event:PB_CONNECT",
        'data:{"clientId":"abc"}',
        "",
        "event:items",
        'data:{"action":"create",',
        'data: "record":{"id":"1"}}',
        "",
    ]
    assert list(_sse(iter(lines))) == [
        ("PB_CONNECT", {"clientId": "abc"}),
        ("items", {"action": "create", "record": {"id": "1"}}),
    ]


def test_settings_are_required():
    with pytest.raises(ValueError, match="PLASTO_WORKER"):
        Client("http://127.0.0.1:1", name="", password="")
