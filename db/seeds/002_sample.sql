-- =============================================================================
-- Sample project seed
--
-- Edit the variables below then run:
--   psql "$DATABASE_URL" -f db/seeds/002_sample.sql
--
-- Or use the CLI helper:
--   ./scripts/load-sample.sh --name myapp --domain myapp.local:8080
-- =============================================================================

-- ── Config (edit these) ─────────────────────────────────────────────
\set project_name   'sample-app'
\set project_domain 'localhost:3000'
\set base_url       'http://localhost:3000'
\set interval_sec   30
\set threshold      3
\set autofix        FALSE
\set fix_name       ''
\set fix_script     ''
\set fix_pattern    ''
\set fix_timeout    300

-- ── Project ─────────────────────────────────────────────────────────
WITH upsert_project AS (
    INSERT INTO projects (name, domain, check_interval_sec, failure_threshold, autofix_enabled, alert_emails)
    VALUES (:'project_name', :'project_domain', :interval_sec, :threshold, :autofix, '{}')
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

-- ── Checks (add/remove rows as needed) ──────────────────────────────
INSERT INTO checks (project_id, type, target, timeout_ms, expected_status)
SELECT p.id, 'http', :'base_url' || '/', 5000, 200
FROM projects p WHERE p.name = :'project_name'
  AND NOT EXISTS (
    SELECT 1 FROM checks c WHERE c.project_id = p.id AND c.target = :'base_url' || '/'
  );

-- ── Fix (optional – remove this block if you don't need autofix) ────
INSERT INTO fixes (name, type, script_path, supported_error_pattern, timeout_sec)
SELECT :'fix_name', 'http', :'fix_script', :'fix_pattern', :fix_timeout
WHERE NOT EXISTS (
    SELECT 1 FROM fixes WHERE name = :'fix_name' AND script_path = :'fix_script'
);

INSERT INTO project_fixes (project_id, fix_id)
SELECT p.id, f.id
FROM projects p
JOIN fixes f ON f.name = :'fix_name' AND f.script_path = :'fix_script'
WHERE p.name = :'project_name'
ON CONFLICT DO NOTHING;

-- ── Summary ─────────────────────────────────────────────────────────
SELECT p.id, p.name, p.domain, p.autofix_enabled, p.check_interval_sec, p.failure_threshold
FROM projects p WHERE p.name = :'project_name';

SELECT c.id, c.type, c.target, c.timeout_ms, c.expected_status
FROM checks c JOIN projects p ON p.id = c.project_id
WHERE p.name = :'project_name' ORDER BY c.id;

SELECT f.id, f.name, f.script_path, f.timeout_sec
FROM fixes f
JOIN project_fixes pf ON pf.fix_id = f.id
JOIN projects p ON p.id = pf.project_id
WHERE p.name = :'project_name';
