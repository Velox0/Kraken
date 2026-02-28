.PHONY: up down migrate api scheduler worker notifier app test

up:
	docker compose up -d

down:
	docker compose down

migrate:
	psql "$$DATABASE_URL" -f db/migrations/0001_init.sql

api:
	go run ./cmd/api

scheduler:
	go run ./cmd/scheduler

worker:
	go run ./cmd/worker

notifier:
	go run ./cmd/notifier

app:
	go run ./cmd/app

test:
	go test ./...
