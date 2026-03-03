.PHONY: up down migrate seed sample api scheduler worker notifier app test \
       setup-postgres setup-redis setup-local

up:
	docker compose up -d

down:
	docker compose down

# ── Local dependency setup (macOS / Linux) ───────────────────────────

setup-postgres:
	@command -v psql >/dev/null 2>&1 || { echo "ERROR: psql not found. Install PostgreSQL first."; exit 1; }
	@echo "==> Starting PostgreSQL..."
	@if [ "$$(uname)" = "Darwin" ]; then \
		brew services start postgresql@16; \
	else \
		sudo systemctl start postgresql; \
	fi
	@sleep 2
	@createdb kraken 2>/dev/null || echo "    db 'kraken' already exists"
	@psql -d kraken -c "SELECT 1" >/dev/null 2>&1 && echo "==> PostgreSQL ready (db: kraken)"

setup-redis:
	@command -v redis-server >/dev/null 2>&1 || { echo "ERROR: redis-server not found. Install Redis first."; exit 1; }
	@echo "==> Starting Redis..."
	@if [ "$$(uname)" = "Darwin" ]; then \
		brew services start redis; \
	else \
		sudo systemctl start redis-server || sudo systemctl start redis; \
	fi
	@sleep 1
	@redis-cli ping >/dev/null 2>&1 && echo "==> Redis ready"

setup-local: setup-postgres setup-redis
	@echo "==> Local services running. Run 'make migrate' next."

migrate:
	psql "$$DATABASE_URL" -f db/migrations/0001_init.sql
	psql "$$DATABASE_URL" -f db/migrations/0002_uptime_rollups.sql
	psql "$$DATABASE_URL" -f db/migrations/0003_autofix_retries.sql
	psql "$$DATABASE_URL" -f db/migrations/0004_email_templates.sql

seed:
	@for f in db/seeds/*.sql; do echo "==> $$f"; psql "$$DATABASE_URL" -f "$$f"; done

sample:
	./scripts/load-sample.sh

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
