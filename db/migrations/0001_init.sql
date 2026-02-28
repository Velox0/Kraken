CREATE TABLE IF NOT EXISTS smtp_profiles (
    id BIGSERIAL PRIMARY KEY,
    host TEXT NOT NULL,
    port INTEGER NOT NULL CHECK (port > 0),
    username TEXT NOT NULL,
    password_encrypted TEXT NOT NULL,
    from_email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS projects (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    domain TEXT NOT NULL,
    check_interval_sec INTEGER NOT NULL CHECK (check_interval_sec > 0),
    failure_threshold INTEGER NOT NULL DEFAULT 3 CHECK (failure_threshold > 0),
    autofix_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    smtp_profile_id BIGINT REFERENCES smtp_profiles(id),
    alert_emails TEXT[] NOT NULL DEFAULT '{}',
    next_check_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS checks (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('http', 'tcp', 'ping')),
    target TEXT NOT NULL,
    timeout_ms INTEGER NOT NULL DEFAULT 5000 CHECK (timeout_ms > 0),
    expected_status INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS incidents (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('open', 'resolved')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    error_message TEXT NOT NULL,
    last_alert_sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS incidents_open_unique
ON incidents(project_id)
WHERE status = 'open';

CREATE TABLE IF NOT EXISTS logs (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS check_runs (
    id BIGSERIAL PRIMARY KEY,
    check_id BIGINT NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('healthy', 'failed')),
    response_time_ms INTEGER,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS project_health (
    project_id BIGINT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_status TEXT NOT NULL DEFAULT 'healthy' CHECK (last_status IN ('healthy', 'failed')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fixes (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    script_path TEXT NOT NULL,
    supported_error_pattern TEXT NOT NULL,
    timeout_sec INTEGER NOT NULL DEFAULT 30 CHECK (timeout_sec > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS project_fixes (
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    fix_id BIGINT NOT NULL REFERENCES fixes(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, fix_id)
);

CREATE INDEX IF NOT EXISTS checks_project_idx ON checks(project_id);
CREATE INDEX IF NOT EXISTS logs_project_idx ON logs(project_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS check_runs_project_idx ON check_runs(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS projects_next_check_idx ON projects(next_check_at);
