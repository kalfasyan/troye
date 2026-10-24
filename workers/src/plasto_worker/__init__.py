"""Write Python workers that talk to a troye app.

from plasto_worker import Client, every

app = Client()  # reads TROYE_URL, TROYE_WORKER and TROYE_WORKER_PASSWORD

def check():
    app.create("items", {"title": "Hello"})

every(60, check, app)
"""

from .client import ApiError, Client
from .loop import every

__all__ = ["ApiError", "Client", "every"]
