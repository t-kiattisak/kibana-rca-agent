#!/usr/bin/env bash
set -e

APP_URL="${APP_URL:-http://localhost:8000}"

echo "=========================================================="
echo "💥 SIMULATING PRODUCTION INCIDENTS ON DEMO APP"
echo "Target URL: $APP_URL"
echo "=========================================================="

echo ""
echo "👉 1. Triggering Database Connection Pool Exhaustion..."
for i in {1..5}; do
  echo "   [Request $i] Calling /api/debug/db-pool-exhaustion"
  curl -s -X GET "$APP_URL/api/debug/db-pool-exhaustion" || true
  echo ""
  sleep 0.2
done

echo ""
echo "👉 2. Triggering 3rd-party Payment Gateway Timeout (HTTP 504)..."
for i in {1..3}; do
  echo "   [Request $i] Calling /api/debug/external-timeout"
  curl -s -X GET "$APP_URL/api/debug/external-timeout" || true
  echo ""
  sleep 0.2
done

echo ""
echo "👉 3. Triggering Unhandled Panic (Nil Pointer Dereference)..."
echo "   [Request 1] Calling /api/debug/panic-nil-pointer"
curl -s -X GET "$APP_URL/api/debug/panic-nil-pointer" || true
echo ""

echo ""
echo "=========================================================="
echo "✅ Finished generating simulated error incidents!"
echo "Now watch the RCA Agent terminal output."
echo "=========================================================="
