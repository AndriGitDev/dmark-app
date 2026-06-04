#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE=${COMPOSE_FILE:-infra/coolify/docker-compose.dev.yml}
DOCKER_COMPOSE=${DOCKER_COMPOSE:-docker compose}
COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-dmark-rc-backend-services}
LOCAL_DB_PASSWORD=${LOCAL_DB_PASSWORD:-meil}
DATABASE_URL=${DATABASE_URL:-postgres://meil:${LOCAL_DB_PASSWORD}@localhost:5432/meil?sslmode=disable}
CLICKHOUSE_URL=${CLICKHOUSE_URL:-http://localhost:8123}
CLICKHOUSE_DATABASE=${CLICKHOUSE_DATABASE:-meil}
CLICKHOUSE_USER=${CLICKHOUSE_USER:-meil}
CLICKHOUSE_PASSWORD=${CLICKHOUSE_PASSWORD:-meil}
RC_BACKEND_SERVICES_TIMEOUT=${RC_BACKEND_SERVICES_TIMEOUT:-90}
RC_BACKEND_SERVICES_NO_COMPOSE=${RC_BACKEND_SERVICES_NO_COMPOSE:-0}

redact() {
  sed -E 's#(postgres://[^:/@]+:)[^@]+@#\1[REDACTED]@#g; s#(password=)[^&[:space:]]+#\1[REDACTED]#gi; s#(CLICKHOUSE_PASSWORD=)[^[:space:]]+#\1[REDACTED]#g'
}

log() {
  printf '[rc-backend-services] %s\n' "$*" | redact
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

run_compose() {
  COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" $DOCKER_COMPOSE -f "$COMPOSE_FILE" "$@"
}

cleanup_compose() {
  if [[ "$RC_BACKEND_SERVICES_NO_COMPOSE" != "1" ]]; then
    run_compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}

wait_for_http() {
  local url=$1
  local deadline=$((SECONDS + RC_BACKEND_SERVICES_TIMEOUT))
  until curl -fsS --max-time 3 "$url" >/dev/null 2>&1; do
    if (( SECONDS >= deadline )); then
      printf 'Timed out waiting for %s\n' "$url" >&2
      return 1
    fi
    sleep 2
  done
}

wait_for_postgres() {
  local deadline=$((SECONDS + RC_BACKEND_SERVICES_TIMEOUT))
  until go run ./cmd/meil migrate postgres status >/dev/null 2>&1; do
    if (( SECONDS >= deadline )); then
      printf 'Timed out waiting for Postgres migration status using configured DATABASE_URL\n' >&2
      return 1
    fi
    sleep 2
  done
}

require_command go
require_command curl

if [[ "$RC_BACKEND_SERVICES_NO_COMPOSE" != "1" ]]; then
  require_command docker
  trap cleanup_compose EXIT
  log "starting fresh service-backed RC dependencies with ${DOCKER_COMPOSE} -f ${COMPOSE_FILE}"
  run_compose down -v --remove-orphans >/dev/null 2>&1 || true
  run_compose up -d postgres clickhouse
else
  log "using external services; not starting compose dependencies"
fi

export DATABASE_URL CLICKHOUSE_URL CLICKHOUSE_DATABASE CLICKHOUSE_USER CLICKHOUSE_PASSWORD

log "waiting for Postgres and ClickHouse readiness"
wait_for_postgres
wait_for_http "${CLICKHOUSE_URL%/}/ping"

log "running fresh Postgres migrations"
go run ./cmd/meil migrate postgres up

log "running fresh ClickHouse migrations"
go run ./cmd/meil migrate clickhouse up

log "running Postgres store integration tests"
MEIL_DB_INTEGRATION=1 go test ./internal/store/pg -count=1

log "service-backed backend RC gates passed"
