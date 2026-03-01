-- Sample fixes
-- These are generic fix scripts. Attach them to a project via the UI or by
-- editing the project_name variable below.

\set project_name 'sample-app'

INSERT INTO fixes (name, type, script_path, supported_error_pattern, timeout_sec)
VALUES
  ('Restart nginx', 'http', 'restart-nginx.sh', 'status code 5[0-9]{2}|connection refused|timeout', 20),
  ('Restart pm2',   'tcp',  'restart-pm2.sh',   'connection refused|reset', 20)
ON CONFLICT DO NOTHING;

-- Attach to project (skip silently if the project doesn't exist yet)
INSERT INTO project_fixes (project_id, fix_id)
SELECT p.id, f.id
FROM projects p, fixes f
WHERE p.name = :'project_name'
  AND f.name IN ('Restart nginx', 'Restart pm2')
ON CONFLICT DO NOTHING;
