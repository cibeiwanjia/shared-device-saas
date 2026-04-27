#!/bin/bash
set -e

EMQX_HOST="${EMQX_HOST:-http://localhost:18083}"
EMQX_USER="${EMQX_USER:-admin}"
EMQX_PASS="${EMQX_PASS:-public}"
JWT_SECRET="${JWT_SECRET:-device-service-access-secret-key}"

echo "Configuring EMQX JWT authentication..."
curl -s -X POST "${EMQX_HOST}/api/v5/authentication" \
  -u "${EMQX_USER}:${EMQX_PASS}" \
  -H "Content-Type: application/json" \
  -d "{
    \"backend\": \"jwt\",
    \"mechanism\": \"jwt\",
    \"jwt\": {
      \"algorithm\": \"hmac-based\",
      \"secret\": \"${JWT_SECRET}\",
      \"secret_base64_encoded\": false
    }
  }" | python3 -m json.tool 2>/dev/null || echo "JWT auth may already exist"

echo ""
echo "EMQX configuration complete."
echo "Dashboard: ${EMQX_HOST} (${EMQX_USER}/${EMQX_PASS})"
