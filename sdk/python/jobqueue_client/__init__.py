"""
JobQueue Python SDK — sync and async clients for the JobQueue HTTP API.

Sync usage::

    from jobqueue_client import JobQueueClient

    client = JobQueueClient("http://localhost:8080", api_key="sk_...")
    job = client.enqueue(type="send_email", payload={"to": "user@example.com"})
    print(job["id"])

Async usage::

    from jobqueue_client import AsyncJobQueueClient
    import asyncio

    async def main():
        client = AsyncJobQueueClient("http://localhost:8080", api_key="sk_...")
        job = await client.enqueue(type="send_email", payload={"to": "user@example.com"})

    asyncio.run(main())
"""

from .client import JobQueueClient
from .async_client import AsyncJobQueueClient
from .exceptions import JobQueueError

__all__ = ["JobQueueClient", "AsyncJobQueueClient", "JobQueueError"]
