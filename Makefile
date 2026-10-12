.PHONY: up down logs dev-db run-backend run-frontend test test-backend test-frontend test-integration lint

DEV_DATABASE_URL ?= postgres://geo:geo@localhost:5432/geo?sslmode=disable

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

dev-db:
	docker compose up -d db

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
	cd backend && DATABASE_URL="$(DEV_DATABASE_URL)" go test -tags=integration ./...

lint:
	cd backend && test -z "$$(gofmt -l .)" && go vet ./...
	cd frontend && npm run lint && npm run typecheck
