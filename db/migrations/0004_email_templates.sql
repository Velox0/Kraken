ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS email_subject_opened TEXT NOT NULL DEFAULT '[DOWN] {domain} is unreachable',
    ADD COLUMN IF NOT EXISTS email_body_opened TEXT NOT NULL DEFAULT E'Project: {project_name}\nDomain: {domain}\nEvent: opened\nIncident ID: {incident_id}\nCheck: #{check_id} {check_type} {check_target}\nError: {error}\nTimestamp: {timestamp}\nAutofix: {autofix_status}',
    ADD COLUMN IF NOT EXISTS email_subject_resolved TEXT NOT NULL DEFAULT '[RESOLVED] {domain} recovered',
    ADD COLUMN IF NOT EXISTS email_body_resolved TEXT NOT NULL DEFAULT E'Project: {project_name}\nDomain: {domain}\nEvent: resolved\nIncident ID: {incident_id}\nCheck: #{check_id} {check_type} {check_target}\nTimestamp: {timestamp}\nAutofix: {autofix_status}',
    ADD COLUMN IF NOT EXISTS email_subject_repeated TEXT NOT NULL DEFAULT '[DOWN][REPEATED] {domain} still failing',
    ADD COLUMN IF NOT EXISTS email_body_repeated TEXT NOT NULL DEFAULT E'Project: {project_name}\nDomain: {domain}\nEvent: repeated\nIncident ID: {incident_id}\nCheck: #{check_id} {check_type} {check_target}\nError: {error}\nTimestamp: {timestamp}\nAutofix: {autofix_status}',
    ADD COLUMN IF NOT EXISTS email_subject_autofix_limit TEXT NOT NULL DEFAULT '[AUTOFIX LIMIT] {domain} retries exhausted',
    ADD COLUMN IF NOT EXISTS email_body_autofix_limit TEXT NOT NULL DEFAULT E'Project: {project_name}\nDomain: {domain}\nIncident ID: {incident_id}\nAutofix attempts: {autofix_attempts}\nMax retries: {max_retries}\nTimestamp: {timestamp}\n\nAutomatic fixes have been exhausted. Manual intervention required.';
