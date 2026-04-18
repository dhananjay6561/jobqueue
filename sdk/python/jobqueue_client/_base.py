"""Shared type aliases and helper used by both sync and async clients."""
from __future__ import annotations

from typing import Any

Job = dict[str, Any]
DLQEntry = dict[str, Any]
Webhook = dict[str, Any]
CronSchedule = dict[str, Any]
APIKey = dict[str, Any]
Stats = dict[str, Any]


def _unwrap(body: dict[str, Any]) -> Any:
    """Extract .data from an API envelope, raising JobQueueError on errors."""
    from .exceptions import JobQueueError
    if body.get("error"):
        raise JobQueueError(body["error"])
    return body.get("data")


def _cursor_page(body: dict[str, Any]) -> dict[str, Any]:
    """Return a cursor page dict as-is (already shaped correctly)."""
    from .exceptions import JobQueueError
    if isinstance(body, dict) and body.get("error"):
        raise JobQueueError(body["error"])
    return body
