#!/bin/sh
set -eu
id="smoke-$(date +%s)"
now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
curl -sf -X POST "http://127.0.0.1:8080/v1/events" \
  -H 'content-type: application/json' \
  -d "{\"id\":\"${id}\",\"type\":\"demo\",\"source\":\"smoke\",\"payload\":{\"ok\":true},\"occurredAt\":\"${now}\"}"
echo
i=0
while [ "$i" -lt 20 ]; do
  body="$(curl -sf -X POST "http://127.0.0.1:8081/query" \
    -H 'content-type: application/json' \
    -d "{\"query\":\"query(\$id: ID!) { event(id: \$id) { id type source } }\",\"variables\":{\"id\":\"${id}\"}}")" || true
  echo "$body"
  echo "$body" | grep -q "\"id\":\"${id}\"" && exit 0
  i=$((i + 1))
  sleep 1
done
echo "event did not appear on the GraphQL edge" >&2
exit 1
