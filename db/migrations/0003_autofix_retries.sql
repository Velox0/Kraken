ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS max_autofix_retries INTEGER NOT NULL DEFAULT 3 CHECK (max_autofix_retries >= 0);

ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS autofix_attempts INTEGER NOT NULL DEFAULT 0;
