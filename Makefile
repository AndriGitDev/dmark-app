.PHONY: dev up down migrate generate test lint build templ-check

up:
	docker compose -f infra/coolify/docker-compose.dev.yml up -d

down:
	docker compose -f infra/coolify/docker-compose.dev.yml down

migrate:
	go run ./cmd/meil migrate up

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

build:
	CGO_ENABLED=0 go build -o bin/meil ./cmd/meil

templ-check:
	templ generate
	git diff --exit-code -- internal/web/views/*_templ.go
