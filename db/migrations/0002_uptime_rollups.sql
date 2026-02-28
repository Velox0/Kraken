CREATE TABLE IF NOT EXISTS project_uptime_state (
    project_id BIGINT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    current_status TEXT NOT NULL CHECK (current_status IN ('up', 'down')),
    cursor_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS project_uptime_minutes (
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    bucket_start TIMESTAMPTZ NOT NULL,
    up_seconds INTEGER NOT NULL DEFAULT 0 CHECK (up_seconds >= 0 AND up_seconds <= 60),
    down_seconds INTEGER NOT NULL DEFAULT 0 CHECK (down_seconds >= 0 AND down_seconds <= 60),
    PRIMARY KEY (project_id, bucket_start)
);

CREATE INDEX IF NOT EXISTS project_uptime_minutes_project_time_idx
ON project_uptime_minutes(project_id, bucket_start DESC);

INSERT INTO project_uptime_state(project_id, current_status, cursor_at, updated_at)
SELECT p.id,
       CASE WHEN EXISTS (
            SELECT 1
            FROM incidents i
            WHERE i.project_id = p.id AND i.status = 'open'
       ) THEN 'down' ELSE 'up' END,
       NOW(),
       NOW()
FROM projects p
ON CONFLICT (project_id) DO NOTHING;
