#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "==> Starting jobqueue (Postgres + Redis + App)..."
docker compose up --build --force-recreate -d

echo "==> Waiting for server to be ready..."
for i in $(seq 1 20); do
  if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    echo ""
    echo "==> Server is UP!"
    curl -s http://localhost:8080/health
    echo ""
    echo "---------------------------------------------"
    echo "  Dashboard : http://localhost:8080"
    echo "  Health    : http://localhost:8080/health"
    echo "  Jobs API  : http://localhost:8080/api/v1/jobs"
    echo "  Stats     : http://localhost:8080/api/v1/stats"
    echo "  Workers   : http://localhost:8080/api/v1/workers"
    echo "---------------------------------------------"
    echo "  To stop   : docker compose down"
    echo "---------------------------------------------"
    exit 0
  fi
  printf "."
  sleep 2
done

echo ""
echo "ERROR: Server did not respond after 40s. Check logs:"
echo "  docker compose logs app"
exit 1
