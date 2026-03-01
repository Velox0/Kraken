#!/usr/bin/env bash
#
# Kraken – setup script (macOS / Linux)
#
# Starts PostgreSQL & Redis, runs migrations, and copies .env.
# Usage: ./scripts/setup.sh

set -euo pipefail
cd "$(dirname "$0")/.."

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${GREEN}==> $1${NC}"; }
warn()  { echo -e "${YELLOW}    $1${NC}"; }
fail()  { echo -e "${RED}ERROR: $1${NC}" >&2; exit 1; }

# ── Pre-flight: Go is required everywhere ────────────────────────────
command -v go >/dev/null 2>&1 || fail "go not found. Install Go first: https://go.dev/dl"

# ── Ask: Docker or local? ───────────────────────────────────────────
echo ""
echo "How do you want to run PostgreSQL and Redis?"
echo "  1) Docker (recommended)"
echo "  2) Local (must already be installed)"
echo ""
read -rp "Choose [1/2]: " CHOICE

case "$CHOICE" in
  2)
    # ── Local ────────────────────────────────────────────────────────
    info "Using local services"

    command -v psql >/dev/null 2>&1 || fail "psql not found. Install PostgreSQL first."
    command -v redis-server >/dev/null 2>&1 || fail "redis-server not found. Install Redis first."

    OS="$(uname)"
    if [ "$OS" = "Darwin" ]; then
      info "Starting PostgreSQL (brew)..."
      brew services start postgresql@16 2>/dev/null || brew services start postgresql 2>/dev/null || true
      info "Starting Redis (brew)..."
      brew services start redis 2>/dev/null || true
    else
      info "Starting PostgreSQL (systemctl)..."
      sudo systemctl start postgresql 2>/dev/null || true
      info "Starting Redis (systemctl)..."
      sudo systemctl start redis-server 2>/dev/null || sudo systemctl start redis 2>/dev/null || true
    fi

    sleep 2
    createdb kraken 2>/dev/null || warn "db 'kraken' already exists"
    psql -d kraken -c "SELECT 1" >/dev/null 2>&1 || fail "PostgreSQL is not responding"
    redis-cli ping >/dev/null 2>&1 || fail "Redis is not responding"
    info "Local PostgreSQL & Redis running"
    ;;
  *)
    # ── Docker (default) ─────────────────────────────────────────────
    info "Using Docker"
    command -v docker >/dev/null 2>&1 || fail "docker not found. Install Docker first: https://docs.docker.com/get-docker"
    docker compose up -d
    info "Waiting for containers..."
    sleep 3
    ;;
esac

# ── .env ─────────────────────────────────────────────────────────────
if [ ! -f .env ]; then
  cp .env.example .env
  info "Created .env from .env.example — edit it with your credentials"
else
  warn ".env already exists, skipping"
fi

# ── Source env for migrations ────────────────────────────────────────
set -a && source .env && set +a

# ── Migrations ───────────────────────────────────────────────────────
info "Running migrations..."
PSQL_CMD=""
if [ "$CHOICE" != "2" ]; then
  # Running via docker
  CONTAINER=$(docker ps -qf "ancestor=postgres:16" 2>/dev/null || true)
  if [ -z "$CONTAINER" ]; then
    CONTAINER=$(docker ps -qf "name=postgres" 2>/dev/null || true)
  fi
  if [ -n "$CONTAINER" ]; then
    PSQL_CMD="docker exec -i $CONTAINER psql -U postgres -d kraken"
  fi
fi

run_sql_file() {
  if [ -n "$PSQL_CMD" ]; then
    $PSQL_CMD < "$1"
  else
    psql "$DATABASE_URL" -f "$1"
  fi
}

for f in db/migrations/*.sql; do
  warn "$(basename "$f")"
  run_sql_file "$f"
done

# ── Seeds ────────────────────────────────────────────────────────────
info "Running seeds..."
for f in db/seeds/*.sql; do
  warn "$(basename "$f")"
  run_sql_file "$f"
done

# ── Done ─────────────────────────────────────────────────────────────
echo ""
info "Setup complete!"
echo ""
echo "  Start the app:    make app"
echo "  Or individually:  make api / make scheduler / make worker / make notifier"
echo ""
