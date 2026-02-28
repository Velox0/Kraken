-- Example fixes
INSERT INTO fixes (name, type, script_path, supported_error_pattern, timeout_sec)
VALUES
  ('Restart nginx', 'http', 'restart-nginx.sh', 'status code 5[0-9]{2}|connection refused|timeout', 20),
  ('Restart pm2', 'tcp', 'restart-pm2.sh', 'connection refused|reset', 20)
ON CONFLICT DO NOTHING;

-- Attach to project 1 (adjust project_id as needed)
INSERT INTO project_fixes(project_id, fix_id)
SELECT 1, id FROM fixes
ON CONFLICT DO NOTHING;
