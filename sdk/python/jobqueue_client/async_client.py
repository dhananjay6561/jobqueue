"""Async JobQueue client (requires httpx with async support)."""
from __future__ import annotations

from typing import Any

from .exceptions import JobQueueError
from ._base import _unwrap, _cursor_page, Job, DLQEntry, Webhook, CronSchedule, APIKey, Stats


class AsyncJobQueueClient:
    """Async HTTP client for the JobQueue API. Use with ``async with`` or call ``await client.aclose()``."""

    def __init__(self, base_url: str, *, api_key: str = "", timeout: float = 30.0) -> None:
        try:
            import httpx
        except ImportError:
            raise ImportError("httpx is required: pip install httpx") from None

        self._base = base_url.rstrip("/")
        self._client = httpx.AsyncClient(
            base_url=self._base,
            headers={"X-API-Key": api_key} if api_key else {},
            timeout=timeout,
        )

    async def aclose(self) -> None:
        await self._client.aclose()

    async def __aenter__(self) -> "AsyncJobQueueClient":
        return self

    async def __aexit__(self, *_: Any) -> None:
        await self.aclose()

    # ── Jobs ──────────────────────────────────────────────────────────────────

    async def enqueue(
        self,
        *,
        type: str,
        payload: dict[str, Any] | None = None,
        priority: int = 5,
        max_attempts: int = 3,
        queue_name: str = "default",
        scheduled_at: str | None = None,
        ttl_seconds: int = 0,
    ) -> Job:
        body: dict[str, Any] = {
            "type": type,
            "payload": payload or {},
            "priority": priority,
            "max_attempts": max_attempts,
            "queue_name": queue_name,
        }
        if scheduled_at:
            body["scheduled_at"] = scheduled_at
        if ttl_seconds:
            body["ttl_seconds"] = ttl_seconds
        return _unwrap(await self._post("/api/v1/jobs", body))

    async def enqueue_batch(self, jobs: list[dict[str, Any]]) -> list[Job]:
        return _unwrap(await self._post("/api/v1/jobs/batch", jobs))

    async def get_job(self, job_id: str) -> Job:
        return _unwrap(await self._get(f"/api/v1/jobs/{job_id}"))

    async def list_jobs(self, *, status: str = "", type: str = "", queue: str = "", limit: int = 20, offset: int = 0) -> dict[str, Any]:
        params: dict[str, Any] = {"limit": limit, "offset": offset}
        if status:
            params["status"] = status
        if type:
            params["type"] = type
        if queue:
            params["queue"] = queue
        return await self._get("/api/v1/jobs", params=params)

    async def list_jobs_cursor(self, *, status: str = "", type: str = "", queue: str = "", cursor: str = "", limit: int = 20) -> dict[str, Any]:
        params: dict[str, Any] = {"limit": limit}
        if status:
            params["status"] = status
        if type:
            params["type"] = type
        if queue:
            params["queue"] = queue
        if cursor:
            params["cursor"] = cursor
        return _cursor_page(await self._get("/api/v1/jobs/cursor", params=params))

    async def cancel_job(self, job_id: str) -> None:
        await self._delete(f"/api/v1/jobs/{job_id}")

    async def retry_job(self, job_id: str) -> Job:
        return _unwrap(await self._post(f"/api/v1/jobs/{job_id}/retry", None))

    async def get_job_result(self, job_id: str) -> Any:
        r = await self._client.get(f"/api/v1/jobs/{job_id}/result")
        if r.status_code == 204:
            return None
        if not r.is_success:
            raise JobQueueError(r.json().get("error", f"HTTP {r.status_code}"), r.status_code)
        return r.json()

    async def purge_jobs(self, *, before: str) -> dict[str, int]:
        r = await self._client.delete("/api/v1/jobs", params={"before": before})
        r.raise_for_status()
        return r.json()

    # ── Stats ─────────────────────────────────────────────────────────────────

    async def get_stats(self) -> Stats:
        return _unwrap(await self._get("/api/v1/stats"))

    # ── DLQ ───────────────────────────────────────────────────────────────────

    async def list_dlq(self, *, limit: int = 20, offset: int = 0) -> dict[str, Any]:
        return await self._get("/api/v1/dlq", params={"limit": limit, "offset": offset})

    async def requeue_dlq(self, dlq_id: str) -> dict[str, Any]:
        return _unwrap(await self._post(f"/api/v1/dlq/{dlq_id}/requeue", None))

    # ── Webhooks ──────────────────────────────────────────────────────────────

    async def list_webhooks(self) -> list[Webhook]:
        return _unwrap(await self._get("/api/v1/webhooks"))

    async def create_webhook(self, *, url: str, secret: str = "", events: list[str] | None = None, enabled: bool = True) -> Webhook:
        return _unwrap(await self._post("/api/v1/webhooks", {"url": url, "secret": secret, "events": events or [], "enabled": enabled}))

    async def delete_webhook(self, webhook_id: str) -> None:
        await self._delete(f"/api/v1/webhooks/{webhook_id}")

    # ── Cron ──────────────────────────────────────────────────────────────────

    async def list_cron(self) -> list[CronSchedule]:
        return _unwrap(await self._get("/api/v1/cron"))

    async def create_cron(self, *, name: str, job_type: str, cron_expression: str, payload: dict[str, Any] | None = None,
                          queue_name: str = "default", priority: int = 5, max_attempts: int = 3, enabled: bool = True) -> CronSchedule:
        return _unwrap(await self._post("/api/v1/cron", {
            "name": name, "job_type": job_type, "cron_expression": cron_expression,
            "payload": payload or {}, "queue_name": queue_name,
            "priority": priority, "max_attempts": max_attempts, "enabled": enabled,
        }))

    async def patch_cron(self, cron_id: str, **fields: Any) -> CronSchedule:
        return _unwrap(await self._patch(f"/api/v1/cron/{cron_id}", fields))

    async def delete_cron(self, cron_id: str) -> None:
        await self._delete(f"/api/v1/cron/{cron_id}")

    # ── API Keys ──────────────────────────────────────────────────────────────

    async def list_api_keys(self) -> list[APIKey]:
        return _unwrap(await self._get("/api/v1/keys"))

    async def create_api_key(self, name: str, tier: str = "free") -> dict[str, Any]:
        return _unwrap(await self._post("/api/v1/keys", {"name": name, "tier": tier}))

    async def delete_api_key(self, key_id: str) -> None:
        await self._delete(f"/api/v1/keys/{key_id}")

    async def get_usage(self) -> APIKey:
        return _unwrap(await self._get("/api/v1/usage"))

    # ── Health ─────────────────────────────────────────────────────────────────

    async def health(self) -> dict[str, Any]:
        r = await self._client.get("/health")
        if not r.is_success:
            raise JobQueueError(f"unhealthy (status {r.status_code})", r.status_code)
        return r.json()

    # ── Internal ───────────────────────────────────────────────────────────────

    async def _get(self, path: str, *, params: dict[str, Any] | None = None) -> Any:
        r = await self._client.get(path, params=params)
        return self._handle(r)

    async def _post(self, path: str, body: Any) -> Any:
        r = await self._client.post(path, json=body)
        return self._handle(r)

    async def _patch(self, path: str, body: Any) -> Any:
        r = await self._client.patch(path, json=body)
        return self._handle(r)

    async def _delete(self, path: str) -> None:
        r = await self._client.delete(path)
        if not r.is_success:
            raise JobQueueError(r.json().get("error", f"HTTP {r.status_code}"), r.status_code)

    def _handle(self, r: Any) -> Any:
        try:
            body = r.json()
        except Exception:
            raise JobQueueError(f"HTTP {r.status_code}: non-JSON response", r.status_code)
        if not r.is_success:
            raise JobQueueError(body.get("error", f"HTTP {r.status_code}"), r.status_code)
        return body
