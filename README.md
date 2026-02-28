# Kraken (MVP Foundation)

Kraken is a project-based uptime monitor with queue-driven execution and incident/alert handling.

## Implemented MVP Foundation

- HTTP/TCP/Ping checks executed by dedicated workers
- Scheduler that enqueues work (checks are never run in API process)
- Incident dedup (`one open incident per project`)
- Consecutive-failure threshold before opening incident
- SMTP alerts for `opened`, `resolved`, and cooldown-based repeated failures
- Autofix plugin model with DB-backed scripts and safe execution constraints
- PostgreSQL schema for projects/checks/incidents/logs/smtp/fixes

## Process Architecture

- `cmd/api`: CRUD + manual trigger API
- `cmd/scheduler`: picks due projects and enqueues check jobs
- `cmd/worker`: executes checks, updates health/incidents, runs autofix, queues alerts
- `cmd/notifier`: sends SMTP alerts from email queue

## Quick Start

1. Start dependencies:

```bash
docker compose up -d
```

2. Configure environment:

```bash
cp .env.example .env
export $(grep -v '^#' .env | xargs)
```

3. Run migration:

```bash
make migrate
```

4. Run services in separate terminals:

```bash
make api
make scheduler
make worker
make notifier
```

## API Endpoints

- `GET /healthz`
- `GET /v1/projects`
- `POST /v1/projects`
- `PATCH /v1/projects/{projectID}/autofix`
- `GET /v1/projects/{projectID}/checks`
- `POST /v1/projects/{projectID}/checks`
- `POST /v1/projects/{projectID}/run-now`
- `POST /v1/smtp_profiles`

### Example: Create SMTP Profile

```bash
curl -X POST http://localhost:8080/v1/smtp_profiles \
  -H 'content-type: application/json' \
  -d '{
    "host":"smtp.example.com",
    "port":587,
    "username":"alerts@example.com",
    "password":"supersecret",
    "from_email":"alerts@example.com"
  }'
```

### Example: Create Project

```bash
curl -X POST http://localhost:8080/v1/projects \
  -H 'content-type: application/json' \
  -d '{
    "name":"Example",
    "domain":"example.com",
    "check_interval_sec":30,
    "failure_threshold":3,
    "autofix_enabled":true,
    "smtp_profile_id":1,
    "alert_emails":["oncall@example.com"]
  }'
```

### Example: Add Check

```bash
curl -X POST http://localhost:8080/v1/projects/1/checks \
  -H 'content-type: application/json' \
  -d '{
    "type":"http",
    "target":"https://example.com",
    "timeout_ms":5000,
    "expected_status":200
  }'
```

## Autofix Safety Baseline

- Script path constrained to `FIX_SCRIPTS_DIR`
- Command allowlist (`ALLOWED_FIX_COMMANDS`)
- Per-fix timeout enforced
- Full stdout/stderr output logged

## Notes

- `smtp_profiles.password_encrypted` currently stores provided value as-is. Integrate KMS/app-key encryption before production.
- Ping checks rely on system `ping` binary availability.
