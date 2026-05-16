.PHONY: dev up down migrate migrate-clickhouse generate test lint security fuzz-smoke build templ-check generate-check qa-code qa-services qa load-checker load-api load-reporter load-scan-job

up:
	docker compose -f infra/coolify/docker-compose.dev.yml up -d

down:
	docker compose -f infra/coolify/docker-compose.dev.yml down

migrate:
	go run ./cmd/meil migrate up

migrate-clickhouse:
	go run ./cmd/meil migrate clickhouse up

generate:
	templ generate
	sqlc generate
	tailwindcss -i assets/css/input.css -o assets/css/output.css

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

load-checker:
	go run ./scripts/load --scenario checker --base-url $${BASE_URL:-http://127.0.0.1:8080} --domain $${DOMAIN:-example.com} --requests $${REQUESTS:-50} --concurrency $${CONCURRENCY:-5}

load-api:
	go run ./scripts/load --scenario api-get --base-url $${BASE_URL:-http://127.0.0.1:8080} --path $${PATH:-/healthz} --requests $${REQUESTS:-50} --concurrency $${CONCURRENCY:-5}

load-reporter:
	go run ./scripts/load --scenario reporter-get --base-url $${BASE_URL:-http://127.0.0.1:8080} --path $${PATH:-/api/v1/domains/$${DOMAIN_ID}/reporter/sources} --api-token $${API_TOKEN} --expect-status $${EXPECT_STATUS:-200} --requests $${REQUESTS:-50} --concurrency $${CONCURRENCY:-5}

load-scan-job:
	go run ./scripts/load --scenario api-post --base-url $${BASE_URL:-http://127.0.0.1:8080} --path $${PATH:-/api/v1/domains/$${DOMAIN_ID}/check} --api-token $${API_TOKEN} --expect-status $${EXPECT_STATUS:-200} --requests $${REQUESTS:-50} --concurrency $${CONCURRENCY:-5}

build:
	CGO_ENABLED=0 go build -o bin/meil ./cmd/meil

templ-check:
	@tmp_dir=$$(mktemp -d); \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	cp internal/web/views/*_templ.go "$$tmp_dir"/; \
	templ generate; \
	for file in internal/web/views/*_templ.go; do \
		base=$$(basename "$$file"); \
		cmp -s "$$tmp_dir/$$base" "$$file" || { echo "$$file changed after templ generate"; exit 1; }; \
	done

generate-check:
	@tmp_dir=$$(mktemp -d); \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	mkdir -p "$$tmp_dir/views" "$$tmp_dir/pg" "$$tmp_dir/css"; \
	cp internal/web/views/*_templ.go "$$tmp_dir/views"/; \
	cp internal/store/pg/*.sql.go internal/store/pg/db.go internal/store/pg/models.go "$$tmp_dir/pg"/; \
	cp assets/css/output.css "$$tmp_dir/css"/; \
	$(MAKE) generate; \
	for file in internal/web/views/*_templ.go; do \
		base=$$(basename "$$file"); \
		cmp -s "$$tmp_dir/views/$$base" "$$file" || { echo "$$file changed after generate"; exit 1; }; \
	done; \
	for file in internal/store/pg/*.sql.go internal/store/pg/db.go internal/store/pg/models.go; do \
		base=$$(basename "$$file"); \
		cmp -s "$$tmp_dir/pg/$$base" "$$file" || { echo "$$file changed after generate"; exit 1; }; \
	done; \
	cmp -s "$$tmp_dir/css/output.css" assets/css/output.css || { echo "assets/css/output.css changed after generate"; exit 1; }

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
