@echo off
REM Kraken – setup script (Windows)
REM
REM Uses Docker for PostgreSQL & Redis, runs migrations, and copies .env.
REM Usage: scripts\setup.bat

cd /d "%~dp0\.."

echo.
echo === Kraken Setup (Windows) ===
echo.

REM ── Pre-flight ──────────────────────────────────────────────────────
where go >nul 2>&1 || (
    echo ERROR: go not found. Install Go first: https://go.dev/dl
    exit /b 1
)

where docker >nul 2>&1 || (
    echo ERROR: docker not found. Install Docker Desktop: https://docs.docker.com/get-docker
    exit /b 1
)

REM ── Start containers ────────────────────────────────────────────────
echo ==^> Starting Docker containers...
docker compose up -d
if errorlevel 1 (
    echo ERROR: docker compose failed
    exit /b 1
)

echo ==^> Waiting for containers...
timeout /t 5 /nobreak >nul

REM ── .env ────────────────────────────────────────────────────────────
if not exist .env (
    copy .env.example .env >nul
    echo ==^> Created .env from .env.example — edit it with your credentials
) else (
    echo     .env already exists, skipping
)

REM ── Find postgres container ─────────────────────────────────────────
for /f "tokens=*" %%i in ('docker ps -qf "ancestor=postgres:16" 2^>nul') do set PG_CONTAINER=%%i
if not defined PG_CONTAINER (
    for /f "tokens=*" %%i in ('docker ps -qf "name=postgres" 2^>nul') do set PG_CONTAINER=%%i
)
if not defined PG_CONTAINER (
    echo ERROR: PostgreSQL container not found
    exit /b 1
)

REM ── Migrations ──────────────────────────────────────────────────────
echo ==^> Running migrations...
for %%f in (db\migrations\*.sql) do (
    echo     %%~nxf
    docker exec -i %PG_CONTAINER% psql -U postgres -d kraken < "%%f"
    if errorlevel 1 (
        echo ERROR: migration %%~nxf failed
        exit /b 1
    )
)

REM ── Seeds ───────────────────────────────────────────────────────────
echo ==^> Running seeds...
for %%f in (db\seeds\*.sql) do (
    echo     %%~nxf
    docker exec -i %PG_CONTAINER% psql -U postgres -d kraken < "%%f"
)

REM ── Done ────────────────────────────────────────────────────────────
echo.
echo ==^> Setup complete!
echo.
echo   Start the app:    make app
echo   Or individually:  make api / make scheduler / make worker / make notifier
echo.
