DOCKER_COMPOSE ?= docker compose

.PHONY: dev up down migrate migrate-clickhouse generate test lint security fuzz-smoke build generate-check qa-code qa-services qa rc-backend load-api load-dashboard load-dashboard-reporter load-dashboard-brand-risk load-dashboard-alerts load-dashboard-settings load-reporter load-scan-job

up:
	$(DOCKER_COMPOSE) -f infra/coolify/docker-compose.dev.yml up -d

down:
	$(DOCKER_COMPOSE) -f infra/coolify/docker-compose.dev.yml down

migrate:
	go run ./cmd/meil migrate up

migrate-clickhouse:
	go run ./cmd/meil migrate clickhouse up

generate:
	sqlc generate

dev: up migrate generate
	air

test:
	go test ./...

lint:
	golangci-lint run

security:
	GOMEMLIMIT=$${GOMEMLIMIT:-512MiB} govulncheck ./...

fuzz-smoke:
	go test ./internal/scanner -run=^$$ -fuzz=FuzzNormalizeDomain -fuzztime=$${FUZZTIME:-2s}
	go test ./internal/scanner -run=^$$ -fuzz=FuzzParseSPF -fuzztime=$${FUZZTIME:-2s}
	go test ./internal/scanner -run=^$$ -fuzz=FuzzParseTagRecord -fuzztime=$${FUZZTIME:-2s}
	go test ./internal/ingest -run=^$$ -fuzz=FuzzParseAggregatePayload -fuzztime=$${FUZZTIME:-2s}

load-api:
	go run ./scripts/load --scenario api-get --base-url $${BASE_URL:-http://127.0.0.1:8080} --path $${PATH:-/healthz} --requests $${REQUESTS:-50} --concurrency $${CONCURRENCY:-5}

load-dashboard:
	go run ./scripts/load --scenario api-get --base-url $${BASE_URL:-http://127.0.0.1:8080} --path $${PATH:-/api/v1/dashboard/domains} --api-token $${API_TOKEN} --expect-status $${EXPECT_STATUS:-200} --requests $${REQUESTS:-50} --concurrency $${CONCURRENCY:-5}

load-dashboard-reporter:
	go run ./scripts/load --scenario reporter-get --base-url $${BASE_URL:-http://127.0.0.1:8080} --path $${PATH:-/api/v1/dashboard/domains/$${DOMAIN_ID}/reporter} --api-token $${API_TOKEN} --expect-status $${EXPECT_STATUS:-200} --requests $${REQUESTS:-50} --concurrency $${CONCURRENCY:-5}

load-dashboard-brand-risk:
	go run ./scripts/load --scenario api-get --base-url $${BASE_URL:-http://127.0.0.1:8080} --path $${PATH:-/api/v1/dashboard/domains/$${DOMAIN_ID}/brand-risk} --api-token $${API_TOKEN} --expect-status $${EXPECT_STATUS:-200} --requests $${REQUESTS:-50} --concurrency $${CONCURRENCY:-5}

load-dashboard-alerts:
	go run ./scripts/load --scenario api-get --base-url $${BASE_URL:-http://127.0.0.1:8080} --path $${PATH:-/api/v1/dashboard/alerts} --api-token $${API_TOKEN} --expect-status $${EXPECT_STATUS:-200} --requests $${REQUESTS:-50} --concurrency $${CONCURRENCY:-5}

load-dashboard-settings:
	go run ./scripts/load --scenario api-get --base-url $${BASE_URL:-http://127.0.0.1:8080} --path $${PATH:-/api/v1/dashboard/settings} --api-token $${API_TOKEN} --expect-status $${EXPECT_STATUS:-200} --requests $${REQUESTS:-50} --concurrency $${CONCURRENCY:-5}

load-reporter:
	go run ./scripts/load --scenario reporter-get --base-url $${BASE_URL:-http://127.0.0.1:8080} --path $${PATH:-/api/v1/domains/$${DOMAIN_ID}/reporter/sources} --api-token $${API_TOKEN} --expect-status $${EXPECT_STATUS:-200} --requests $${REQUESTS:-50} --concurrency $${CONCURRENCY:-5}

load-scan-job:
	go run ./scripts/load --scenario api-post --base-url $${BASE_URL:-http://127.0.0.1:8080} --path $${PATH:-/api/v1/domains/$${DOMAIN_ID}/check} --api-token $${API_TOKEN} --expect-status $${EXPECT_STATUS:-200} --requests $${REQUESTS:-50} --concurrency $${CONCURRENCY:-5}

build:
	CGO_ENABLED=0 go build -o bin/meil ./cmd/meil

generate-check:
	@set -e; \
	tmp_dir=$$(mktemp -d); \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	mkdir -p "$$tmp_dir/pg"; \
	cp internal/store/pg/*.sql.go internal/store/pg/db.go internal/store/pg/models.go "$$tmp_dir/pg"/; \
	$(MAKE) generate; \
	for file in internal/store/pg/*.sql.go internal/store/pg/db.go internal/store/pg/models.go; do \
		base=$$(basename "$$file"); \
		cmp -s "$$tmp_dir/pg/$$base" "$$file" || { echo "$$file changed after generate"; exit 1; }; \
	done

qa-code: test
	go test -race ./...
	go vet ./...
	golangci-lint run
	$(MAKE) security
	$(MAKE) fuzz-smoke
	$(MAKE) generate-check
	$(MAKE) build

qa-services:
	docker build -f infra/coolify/Dockerfile .
	go run ./cmd/meil migrate postgres up
	go run ./cmd/meil migrate clickhouse up

qa: qa-code qa-services

rc-backend:
	go test ./... -count=1
	go test -race ./... -count=1
	go vet ./...
	golangci-lint run ./...
	$(MAKE) security
	$(MAKE) generate-check
	go build ./cmd/meil
	$(DOCKER_COMPOSE) -f infra/coolify/docker-compose.dev.yml config --quiet
	docker build -f infra/coolify/Dockerfile .
