// src/index.ts
var JobQueueError = class extends Error {
  constructor(message, status) {
    super(message);
    this.status = status;
    this.name = "JobQueueError";
  }
};
var JobQueueClient = class {
  constructor(baseURL, options = {}) {
    this.baseURL = baseURL.replace(/\/$/, "");
    this.apiKey = options.apiKey ?? "";
    this.timeout = options.timeout ?? 3e4;
    this._fetch = options.fetch ?? globalThis.fetch.bind(globalThis);
  }
  // ── Jobs ────────────────────────────────────────────────────────────────────
  async enqueue(req) {
    const body = {
      priority: 5,
      max_attempts: 3,
      queue_name: "default",
      ...req
    };
    return this._post("/api/v1/jobs", body);
  }
  async enqueueBatch(reqs) {
    return this._post("/api/v1/jobs/batch", reqs);
  }
  async getJob(id) {
    return this._get(`/api/v1/jobs/${id}`);
  }
  async listJobs(params = {}) {
    const q = new URLSearchParams();
    if (params.status) q.set("status", params.status);
    if (params.type) q.set("type", params.type);
    if (params.queue) q.set("queue", params.queue);
    q.set("limit", String(params.limit ?? 20));
    q.set("offset", String(params.offset ?? 0));
    return this._getPage(`/api/v1/jobs?${q}`);
  }
  async cancelJob(id) {
    return this._delete(`/api/v1/jobs/${id}`);
  }
  async retryJob(id) {
    return this._post(`/api/v1/jobs/${id}/retry`, null);
  }
  async getJobResult(id) {
    const res = await this._request("GET", `/api/v1/jobs/${id}/result`);
    if (res.status === 204) return null;
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new JobQueueError(body.error ?? `HTTP ${res.status}`, res.status);
    }
    return res.json();
  }
  // ── Stats ───────────────────────────────────────────────────────────────────
  async getStats() {
    return this._get("/api/v1/stats");
  }
  // ── DLQ ─────────────────────────────────────────────────────────────────────
  async listDLQ(limit = 20, offset = 0) {
    return this._getPage(`/api/v1/dlq?limit=${limit}&offset=${offset}`);
  }
  async requeueDLQ(dlqId) {
    return this._post(`/api/v1/dlq/${dlqId}/requeue`, null);
  }
  // ── Webhooks ─────────────────────────────────────────────────────────────────
  async listWebhooks() {
    return this._get("/api/v1/webhooks");
  }
  async createWebhook(req) {
    return this._post("/api/v1/webhooks", req);
  }
  async deleteWebhook(id) {
    return this._delete(`/api/v1/webhooks/${id}`);
  }
  // ── Cron ─────────────────────────────────────────────────────────────────────
  async listCron() {
    return this._get("/api/v1/cron");
  }
  async createCron(req) {
    return this._post("/api/v1/cron", req);
  }
  async patchCron(id, patch) {
    return this._patch(`/api/v1/cron/${id}`, patch);
  }
  async deleteCron(id) {
    return this._delete(`/api/v1/cron/${id}`);
  }
  // ── API Keys ──────────────────────────────────────────────────────────────────
  async listAPIKeys() {
    return this._get("/api/v1/keys");
  }
  async createAPIKey(name, tier = "free") {
    return this._post("/api/v1/keys", { name, tier });
  }
  async deleteAPIKey(id) {
    return this._delete(`/api/v1/keys/${id}`);
  }
  async getUsage() {
    return this._get("/api/v1/usage");
  }
  // ── Health ────────────────────────────────────────────────────────────────────
  async health() {
    const res = await this._request("GET", "/health");
    if (!res.ok) throw new JobQueueError(`unhealthy (status ${res.status})`, res.status);
    return res.json();
  }
  // ── Internal helpers ───────────────────────────────────────────────────────────
  _headers() {
    const h = {
      "Content-Type": "application/json",
      Accept: "application/json"
    };
    if (this.apiKey) h["X-API-Key"] = this.apiKey;
    return h;
  }
  async _request(method, path, body) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeout);
    try {
      return await this._fetch(this.baseURL + path, {
        method,
        headers: this._headers(),
        body: body != null ? JSON.stringify(body) : void 0,
        signal: controller.signal
      });
    } finally {
      clearTimeout(timer);
    }
  }
  async _unwrap(res) {
    const envelope = await res.json();
    if (envelope.error) throw new JobQueueError(envelope.error, res.status);
    return envelope.data;
  }
  async _get(path) {
    const res = await this._request("GET", path);
    return this._unwrap(res);
  }
  async _getPage(path) {
    const res = await this._request("GET", path);
    const envelope = await res.json();
    if (envelope.error) throw new JobQueueError(envelope.error, res.status);
    return {
      items: envelope.data ?? [],
      total_count: envelope.meta?.total_count ?? 0,
      limit: envelope.meta?.limit ?? 20,
      offset: envelope.meta?.offset ?? 0,
      has_more: envelope.meta?.has_more ?? false
    };
  }
  async _post(path, body) {
    const res = await this._request("POST", path, body);
    return this._unwrap(res);
  }
  async _patch(path, body) {
    const res = await this._request("PATCH", path, body);
    return this._unwrap(res);
  }
  async _delete(path) {
    const res = await this._request("DELETE", path);
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new JobQueueError(body.error ?? `HTTP ${res.status}`, res.status);
    }
  }
};
var index_default = JobQueueClient;
export {
  JobQueueClient,
  JobQueueError,
  index_default as default
};
