.PHONY: up down logs dev-db seed run-backend run-frontend test test-backend test-frontend test-integration lint fetch-osm

-include .env

POSTGRES_USER ?= geo
POSTGRES_PASSWORD ?= geo
POSTGRES_DB ?= geo
DB_PORT ?= 5432
DEV_DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(DB_PORT)/$(POSTGRES_DB)?sslmode=disable

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

dev-db:
	docker compose up -d --wait db
	docker compose run --rm --build seed

seed:
	docker compose run --rm --build seed

run-backend:
	cd backend && DATABASE_URL="$(DEV_DATABASE_URL)" go run ./cmd/server

run-frontend:
	cd frontend && npm run dev

test: test-backend test-frontend

test-backend:
	cd backend && go test ./...

test-frontend:
	cd frontend && npm test

test-integration:
	cd backend && TEST_DATABASE_URL="$(DEV_DATABASE_URL)" go test -count=1 -tags=integration ./...

lint:
	cd backend && unformatted="$$("$$(go env GOROOT)/bin/gofmt" -l .)" && if [ -n "$$unformatted" ]; then echo "以下文件需要 gofmt：" >&2; echo "$$unformatted" >&2; exit 1; fi && go vet -tags=integration ./...
	cd frontend && npm run lint && npm run typecheck

fetch-osm:
	./scripts/fetch_osm_pois.sh
