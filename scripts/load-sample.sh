#!/usr/bin/env bash
#
# Load a sample project into Kraken.
#
# Usage:
#   ./scripts/load-sample.sh                              # uses defaults (veloxzerror-local)
#   ./scripts/load-sample.sh --name myapp --domain myapp.local:8080
#   ./scripts/load-sample.sh --name myapp --domain myapp.local:8080 --checks "/,/api/health,/login"
#
# Environment:
#   DATABASE_URL   – Postgres connection string (reads .env if present)
#   PSQL_CMD       – override psql binary (default: auto-detect docker vs local)

set -euo pipefail
cd "$(dirname "$0")/.."

# ── Load .env if present ─────────────────────────────────────────────
if [[ ! -f .env && -f .env.example ]]; then
  cp .env.example .env
  echo "==> Created .env from .env.example"
fi
[[ -f .env ]] && set -a && source .env && set +a

# ── Defaults ─────────────────────────────────────────────────────────
PROJECT_NAME="sample-app"
DOMAIN="localhost:3000"
SCHEME="http"
CHECK_PATHS="/"
INTERVAL=30
THRESHOLD=3
AUTOFIX=TRUE
FIX_NAME=""
FIX_SCRIPT=""
FIX_PATTERN=""
FIX_TIMEOUT=300

# ── Parse args ───────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --name)        PROJECT_NAME="$2"; shift 2 ;;
    --domain)      DOMAIN="$2"; shift 2 ;;
    --scheme)      SCHEME="$2"; shift 2 ;;
    --checks)      CHECK_PATHS="$2"; shift 2 ;;
    --interval)    INTERVAL="$2"; shift 2 ;;
    --threshold)   THRESHOLD="$2"; shift 2 ;;
    --autofix)     AUTOFIX="$2"; shift 2 ;;
    --fix-name)    FIX_NAME="$2"; shift 2 ;;
    --fix-script)  FIX_SCRIPT="$2"; shift 2 ;;
    --fix-pattern) FIX_PATTERN="$2"; shift 2 ;;
    --fix-timeout) FIX_TIMEOUT="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,/^$/s/^# *//p' "$0"
      exit 0
      ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

DATABASE_URL="${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/kraken?sslmode=disable}"

# ── Resolve psql ─────────────────────────────────────────────────────
if [[ -n "${PSQL_CMD:-}" ]]; then
  PSQL="$PSQL_CMD"
elif command -v psql &>/dev/null; then
  PSQL="psql"
else
  CONTAINER=$(docker ps -qf "ancestor=postgres:16" 2>/dev/null || true)
  if [[ -z "$CONTAINER" ]]; then
    CONTAINER=$(docker ps -qf "name=postgres" 2>/dev/null || true)
  fi
  if [[ -z "$CONTAINER" ]]; then
    echo "ERROR: psql not found and no postgres docker container running." >&2
    exit 1
  fi
  PSQL="docker exec -i $CONTAINER psql -U postgres -d kraken"
fi

run_sql() {
  if [[ "$PSQL" == psql ]]; then
    psql "$DATABASE_URL" <<< "$1"
  else
    echo "$1" | $PSQL
  fi
}

# ── Build base URL ───────────────────────────────────────────────────
BASE_URL="${SCHEME}://${DOMAIN}"

# ── Build check values ───────────────────────────────────────────────
IFS=',' read -ra PATHS <<< "$CHECK_PATHS"
CHECKS_SQL=""
for p in "${PATHS[@]}"; do
  p="$(echo "$p" | xargs)"  # trim whitespace
  TARGET="${BASE_URL}${p}"
  CHECKS_SQL+="
INSERT INTO checks (project_id, type, target, timeout_ms, expected_status)
SELECT p.id, 'http', '${TARGET}', 5000, 200
FROM projects p
WHERE p.name = '${PROJECT_NAME}'
  AND NOT EXISTS (
    SELECT 1 FROM checks c
    WHERE c.project_id = p.id AND c.type = 'http' AND c.target = '${TARGET}'
  );
"
done

# ── Build fix SQL (optional) ─────────────────────────────────────────
FIX_SQL=""
if [[ -n "$FIX_NAME" && -n "$FIX_SCRIPT" ]]; then
  PATTERN="${FIX_PATTERN:-connection refused|status code 5[0-9]\{2\}|timeout|dial tcp}"
  FIX_SQL="
INSERT INTO fixes (name, type, script_path, supported_error_pattern, timeout_sec)
SELECT '${FIX_NAME}', 'http', '${FIX_SCRIPT}', '${PATTERN}', ${FIX_TIMEOUT}
WHERE NOT EXISTS (
    SELECT 1 FROM fixes
    WHERE name = '${FIX_NAME}' AND script_path = '${FIX_SCRIPT}'
);

INSERT INTO project_fixes (project_id, fix_id)
SELECT p.id, f.id
FROM projects p
JOIN fixes f ON f.name = '${FIX_NAME}' AND f.script_path = '${FIX_SCRIPT}'
WHERE p.name = '${PROJECT_NAME}'
ON CONFLICT DO NOTHING;
"
fi

# ── Assemble & execute ───────────────────────────────────────────────
SQL="
-- Upsert project
WITH upsert_project AS (
    INSERT INTO projects (name, domain, check_interval_sec, failure_threshold, autofix_enabled, alert_emails)
    VALUES ('${PROJECT_NAME}', '${DOMAIN}', ${INTERVAL}, ${THRESHOLD}, ${AUTOFIX}, '{}')
    ON CONFLICT (name)
    DO UPDATE SET
        domain = EXCLUDED.domain,
        check_interval_sec = EXCLUDED.check_interval_sec,
        failure_threshold = EXCLUDED.failure_threshold,
        autofix_enabled = EXCLUDED.autofix_enabled
    RETURNING id
)
INSERT INTO project_health(project_id)
SELECT id FROM upsert_project
ON CONFLICT (project_id) DO NOTHING;

-- Checks
${CHECKS_SQL}

-- Fixes
${FIX_SQL}

-- Summary
SELECT p.id, p.name, p.domain, p.autofix_enabled, p.check_interval_sec, p.failure_threshold
FROM projects p WHERE p.name = '${PROJECT_NAME}';

SELECT c.id, c.type, c.target, c.timeout_ms, c.expected_status
FROM checks c JOIN projects p ON p.id = c.project_id
WHERE p.name = '${PROJECT_NAME}' ORDER BY c.id;
"

echo "==> Loading sample: ${PROJECT_NAME} (${DOMAIN})"
run_sql "$SQL"
echo "==> Done."
