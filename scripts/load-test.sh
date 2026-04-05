#!/usr/bin/env bash
set -euo pipefail

LB="${1:-http://localhost:8080}"

echo "=== Phase 1: baseline traffic (100 sequential requests) ==="
for i in $(seq 1 100); do
  curl -s "$LB" > /dev/null
done
echo "Done."

echo ""
echo "=== Phase 2: concurrent slow requests (active connections) ==="
echo "Firing 14 requests to /slow in parallel — each holds a connection for 25s."
echo "Watch the Active Connections chart in Grafana."
for i in $(seq 1 14); do
  curl -s "$LB/slow" > /dev/null &
done
wait
echo "Done."

echo ""
echo "=== Phase 3: inject failures ==="
echo "Discovering registered servers..."
SERVERS=$(curl -s "$LB/servers" | grep -o '"address":"[^"]*"' | cut -d'"' -f4)

echo "Removing all servers from the pool..."
for addr in $SERVERS; do
  curl -s -X POST "$LB/remove" -H "Content-Type: application/json" -d "{\"addr\":\"$addr\"}" > /dev/null
done

echo "Sending 10 requests with no backends (expect 503s)..."
for i in $(seq 1 10); do
  curl -s "$LB" > /dev/null
done

echo "Restoring servers..."
for addr in $SERVERS; do
  curl -s -X POST "$LB/add" -H "Content-Type: application/json" -d "{\"addr\":\"$addr\"}" > /dev/null
done
echo "Done."

echo ""
echo "=== Phase 4: simulate unhealthy server ==="
echo "Setting server1 to unhealthy..."
curl -s "http://localhost:8081/sethealthy?healthy=false" > /dev/null
echo "Waiting for health check to detect it (15s)..."
sleep 15
echo "Watch the Server Health panel in Grafana — server1 should be red."
sleep 10
echo "Restoring server1..."
curl -s "http://localhost:8081/sethealthy?healthy=true" > /dev/null
echo "Done."

echo ""
echo "=== Complete. Open http://localhost:3000 to view the Grafana dashboard. ==="
