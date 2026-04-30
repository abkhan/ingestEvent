#!/usr/bin/env bash
set -euo pipefail

URL=${1:-http://127.0.0.1:30080}
API_KEY=${2:-default-key}
AUTH_HEADER="Authorization: Bearer $API_KEY"

echo "Testing Fulcrum service at $URL"

function check_code() {
  local code="$1"
  local expected="$2"
  local desc="$3"
  if [[ "$code" != "$expected" ]]; then
    echo "ERROR: $desc returned $code, expected $expected"
    exit 1
  fi
}

function curl_with_code() {
  local method="$1"
  local path="$2"
  local data="$3"
  local tmpfile
  tmpfile=$(mktemp)
  local code

  if [[ -n "$data" ]]; then
    code=$(curl -sS -w "%{http_code}" -o "$tmpfile" -H "$AUTH_HEADER" -H "Content-Type: application/json" -X "$method" "$URL$path" --data "$data")
  else
    code=$(curl -sS -w "%{http_code}" -o "$tmpfile" -H "$AUTH_HEADER" -X "$method" "$URL$path")
  fi

  echo "$tmpfile:$code"
}

# Health check
result=$(curl_with_code GET /healthz "")
file=${result%%:*}
code=${result##*:}
check_code "$code" 200 "health check"
echo "OK: health check passed"
echo "$(cat "$file")"
rm -f "$file"

# Metrics check
result=$(curl -sS -w "%{http_code}" -o /tmp/metrics.out "$URL/metrics")
code=${result}
check_code "$code" 200 "metrics endpoint"
if ! grep -q "ingest_events_total" /tmp/metrics.out; then
  echo "ERROR: metrics endpoint did not return expected metric text"
  exit 1
fi
echo "OK: metrics endpoint passed"
rm -f /tmp/metrics.out

# Single event ingestion
single_json='{"event_id":"event-1","tenant_id":"tenant1","user_id":"user1","session_id":"sess1","event_type":"page_view","properties":{"page":"home"},"occurred_at":"2026-04-29T12:00:00Z"}'
result=$(curl_with_code POST /v1/events "$single_json")
file=${result%%:*}
code=${result##*:}
check_code "$code" 202 "single event ingestion"
echo "OK: single event ingestion passed"
rm -f "$file"

# Batch ingestion
batch_json='{"events":[{"event_id":"event-2","tenant_id":"tenant1","user_id":"user1","session_id":"sess2","event_type":"product_click","properties":{"sku":"sku123"},"occurred_at":"2026-04-29T12:01:00Z"}]}'
result=$(curl_with_code POST /v1/events/batch "$batch_json")
file=${result%%:*}
code=${result##*:}
check_code "$code" 200 "batch ingestion"
if ! grep -q '"status"[[:space:]]*:[[:space:]]*"success"' "$file"; then
  echo "ERROR: batch ingestion response did not contain success"
  cat "$file"
  rm -f "$file"
  exit 1
fi
rm -f "$file"
echo "OK: batch ingestion passed"

echo "All checks passed. Fulcrum is available at $URL"
