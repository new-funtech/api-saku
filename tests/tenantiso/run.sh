#!/bin/bash
# Runs the tenant-isolation tests against a throwaway PostgreSQL container and a freshly built API.
# Local only: it never reads a real .env and refuses to run if the target is not loopback.
#   tests/tenantiso/run.sh            run everything
#   tests/tenantiso/run.sh -run TestListsNeverContainAnotherCompany
set -u
cd "$(dirname "$0")/../.."
PGC=hrmis-tenantiso-pg; PGPORT=${TENANTISO_PG_PORT:-55433}; APIPORT=${TENANTISO_API_PORT:-4020}
BIN=$(mktemp -d)/api
cleanup() { [ -n "${TENANTISO_KEEP:-}" ] && { echo "kept: API pid $API_PID on :$APIPORT, postgres $PGC on :$PGPORT"; return; }; [ -n "${API_PID:-}" ] && kill "$API_PID" 2>/dev/null; docker rm -f $PGC >/dev/null 2>&1; }
trap cleanup EXIT

docker rm -f $PGC >/dev/null 2>&1
docker run -d --name $PGC -e POSTGRES_PASSWORD=loadtest -e POSTGRES_DB=hrmis_loadtest -p 127.0.0.1:$PGPORT:5432 postgres:16-alpine >/dev/null || exit 1
GOTOOLCHAIN=${GOTOOLCHAIN:-go1.25.0} go build -o "$BIN" ./cmd || exit 1
for i in $(seq 1 30); do docker exec $PGC pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done; sleep 2

APP_PORT=$APIPORT APP_ENV=tenantiso DB_HOST=127.0.0.1 DB_PORT=$PGPORT DB_USER=postgres DB_PASSWORD=loadtest DB_NAME=hrmis_loadtest DB_SSLMODE=disable \
REDIS_HOST=127.0.0.1 REDIS_PORT=6399 JWT_SECRET=loadtest-secret-not-for-production JWT_EXPIRATION=24h CORS_ORIGINS='*' \
SCOPE_CACHE_TTL_SECONDS=0 RATE_LIMIT_IP_PER_MIN=100000000 AWS_ENDPOINT=http://127.0.0.1:9 AWS_ACCESS_KEY_ID=x AWS_SECRET_ACCESS_KEY=x AWS_BUCKET=none \
RABBITMQ_HOST= MAIL_HOST= FIREBASE_CREDENTIALS_BASE64= PAYROLL_CRON_ENABLED=false \
"$BIN" > "${TENANTISO_API_LOG:-/tmp/tenantiso-api.log}" 2>&1 &
API_PID=$!
for i in $(seq 1 40); do curl -sf localhost:$APIPORT/health >/dev/null && break; sleep 1; done

TENANTISO_BASE_URL=http://127.0.0.1:$APIPORT TENANTISO_DB=127.0.0.1:$PGPORT GOTOOLCHAIN=${GOTOOLCHAIN:-go1.25.0} go test -count=1 ${TENANTISO_FLAGS:-} ./tests/tenantiso/ "$@"
