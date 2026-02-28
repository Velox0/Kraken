package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, postgresURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, postgresURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

type Project struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Domain           string    `json:"domain"`
	CheckIntervalSec int       `json:"check_interval_sec"`
	FailureThreshold int       `json:"failure_threshold"`
	AutofixEnabled   bool      `json:"autofix_enabled"`
	SMTPProfileID    *int64    `json:"smtp_profile_id,omitempty"`
	AlertEmails      []string  `json:"alert_emails"`
	NextCheckAt      time.Time `json:"next_check_at"`
	CreatedAt        time.Time `json:"created_at"`
}

type CreateProjectParams struct {
	Name             string   `json:"name"`
	Domain           string   `json:"domain"`
	CheckIntervalSec int      `json:"check_interval_sec"`
	FailureThreshold int      `json:"failure_threshold"`
	AutofixEnabled   bool     `json:"autofix_enabled"`
	SMTPProfileID    *int64   `json:"smtp_profile_id"`
	AlertEmails      []string `json:"alert_emails"`
}

func (s *Store) CreateProject(ctx context.Context, p CreateProjectParams) (Project, error) {
	if p.FailureThreshold <= 0 {
		p.FailureThreshold = 3
	}
	if p.AlertEmails == nil {
		p.AlertEmails = []string{}
	}
	var project Project
	var smtp sql.NullInt64
	if p.SMTPProfileID != nil {
		smtp = sql.NullInt64{Int64: *p.SMTPProfileID, Valid: true}
	}
	query := `
		INSERT INTO projects (name, domain, check_interval_sec, failure_threshold, autofix_enabled, smtp_profile_id, alert_emails)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, domain, check_interval_sec, failure_threshold, autofix_enabled, smtp_profile_id, alert_emails, next_check_at, created_at
	`
	var smtpID sql.NullInt64
	err := s.pool.QueryRow(ctx, query,
		strings.TrimSpace(p.Name),
		strings.TrimSpace(p.Domain),
		p.CheckIntervalSec,
		p.FailureThreshold,
		p.AutofixEnabled,
		nullInt64Arg(smtp),
		p.AlertEmails,
	).Scan(
		&project.ID,
		&project.Name,
		&project.Domain,
		&project.CheckIntervalSec,
		&project.FailureThreshold,
		&project.AutofixEnabled,
		&smtpID,
		&project.AlertEmails,
		&project.NextCheckAt,
		&project.CreatedAt,
	)
	if err != nil {
		return Project{}, err
	}
	if smtpID.Valid {
		project.SMTPProfileID = &smtpID.Int64
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO project_health(project_id) VALUES($1) ON CONFLICT DO NOTHING`, project.ID)
	if err != nil {
		return Project{}, err
	}
	return project, nil
}

func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	query := `
		SELECT id, name, domain, check_interval_sec, failure_threshold, autofix_enabled, smtp_profile_id, alert_emails, next_check_at, created_at
		FROM projects
		ORDER BY id ASC
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		var p Project
		var smtpID sql.NullInt64
		if err := rows.Scan(&p.ID, &p.Name, &p.Domain, &p.CheckIntervalSec, &p.FailureThreshold, &p.AutofixEnabled, &smtpID, &p.AlertEmails, &p.NextCheckAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		if smtpID.Valid {
			p.SMTPProfileID = &smtpID.Int64
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *Store) SetProjectAutofix(ctx context.Context, projectID int64, enabled bool) error {
	cmd, err := s.pool.Exec(ctx, `UPDATE projects SET autofix_enabled=$2 WHERE id=$1`, projectID, enabled)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("project %d not found", projectID)
	}
	return nil
}

type Check struct {
	ID             int64     `json:"id"`
	ProjectID      int64     `json:"project_id"`
	Type           string    `json:"type"`
	Target         string    `json:"target"`
	TimeoutMs      int       `json:"timeout_ms"`
	ExpectedStatus *int      `json:"expected_status,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateCheckParams struct {
	ProjectID      int64  `json:"project_id"`
	Type           string `json:"type"`
	Target         string `json:"target"`
	TimeoutMs      int    `json:"timeout_ms"`
	ExpectedStatus *int   `json:"expected_status"`
}

func (s *Store) CreateCheck(ctx context.Context, p CreateCheckParams) (Check, error) {
	if p.TimeoutMs <= 0 {
		p.TimeoutMs = 5000
	}
	if p.Type != "http" && p.Type != "tcp" && p.Type != "ping" {
		return Check{}, fmt.Errorf("unsupported check type: %s", p.Type)
	}

	query := `
		INSERT INTO checks (project_id, type, target, timeout_ms, expected_status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, project_id, type, target, timeout_ms, expected_status, created_at
	`
	var c Check
	var expected sql.NullInt32
	err := s.pool.QueryRow(ctx, query,
		p.ProjectID,
		p.Type,
		strings.TrimSpace(p.Target),
		p.TimeoutMs,
		nullIntArg(p.ExpectedStatus),
	).Scan(&c.ID, &c.ProjectID, &c.Type, &c.Target, &c.TimeoutMs, &expected, &c.CreatedAt)
	if err != nil {
		return Check{}, err
	}
	if expected.Valid {
		v := int(expected.Int32)
		c.ExpectedStatus = &v
	}
	return c, nil
}

func (s *Store) ListChecksByProject(ctx context.Context, projectID int64) ([]Check, error) {
	query := `
		SELECT id, project_id, type, target, timeout_ms, expected_status, created_at
		FROM checks
		WHERE project_id=$1
		ORDER BY id ASC
	`
	rows, err := s.pool.Query(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	checks := make([]Check, 0)
	for rows.Next() {
		var c Check
		var expected sql.NullInt32
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Type, &c.Target, &c.TimeoutMs, &expected, &c.CreatedAt); err != nil {
			return nil, err
		}
		if expected.Valid {
			v := int(expected.Int32)
			c.ExpectedStatus = &v
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}

type DueProject struct {
	ID               int64
	CheckIntervalSec int
}

func (s *Store) AcquireDueProjects(ctx context.Context, limit int) ([]DueProject, error) {
	if limit <= 0 {
		limit = 100
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, check_interval_sec
		FROM projects
		WHERE next_check_at <= NOW()
		ORDER BY next_check_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	due := make([]DueProject, 0, limit)
	for rows.Next() {
		var p DueProject
		if err := rows.Scan(&p.ID, &p.CheckIntervalSec); err != nil {
			return nil, err
		}
		due = append(due, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, p := range due {
		_, err := tx.Exec(ctx, `
			UPDATE projects
			SET next_check_at = NOW() + make_interval(secs => $2)
			WHERE id=$1
		`, p.ID, p.CheckIntervalSec)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return due, nil
}

func (s *Store) ListChecksForProjects(ctx context.Context, projectIDs []int64) ([]Check, error) {
	if len(projectIDs) == 0 {
		return nil, nil
	}
	query := `
		SELECT id, project_id, type, target, timeout_ms, expected_status, created_at
		FROM checks
		WHERE project_id = ANY($1)
		ORDER BY id ASC
	`
	rows, err := s.pool.Query(ctx, query, projectIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	checks := make([]Check, 0)
	for rows.Next() {
		var c Check
		var expected sql.NullInt32
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Type, &c.Target, &c.TimeoutMs, &expected, &c.CreatedAt); err != nil {
			return nil, err
		}
		if expected.Valid {
			v := int(expected.Int32)
			c.ExpectedStatus = &v
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}

type CheckContext struct {
	Check
	ProjectName      string
	ProjectDomain    string
	FailureThreshold int
	AutofixEnabled   bool
	ProjectSMTPID    *int64
	AlertEmails      []string
	CheckIntervalSec int
	ProjectNextCheck time.Time
	ProjectCreatedAt time.Time
}

func (s *Store) GetCheckContext(ctx context.Context, checkID int64) (CheckContext, error) {
	query := `
		SELECT c.id, c.project_id, c.type, c.target, c.timeout_ms, c.expected_status, c.created_at,
		       p.name, p.domain, p.failure_threshold, p.autofix_enabled, p.smtp_profile_id, p.alert_emails,
		       p.check_interval_sec, p.next_check_at, p.created_at
		FROM checks c
		JOIN projects p ON p.id = c.project_id
		WHERE c.id = $1
	`
	var r CheckContext
	var expected sql.NullInt32
	var smtp sql.NullInt64
	err := s.pool.QueryRow(ctx, query, checkID).Scan(
		&r.ID,
		&r.ProjectID,
		&r.Type,
		&r.Target,
		&r.TimeoutMs,
		&expected,
		&r.CreatedAt,
		&r.ProjectName,
		&r.ProjectDomain,
		&r.FailureThreshold,
		&r.AutofixEnabled,
		&smtp,
		&r.AlertEmails,
		&r.CheckIntervalSec,
		&r.ProjectNextCheck,
		&r.ProjectCreatedAt,
	)
	if err != nil {
		return CheckContext{}, err
	}
	if expected.Valid {
		v := int(expected.Int32)
		r.ExpectedStatus = &v
	}
	if smtp.Valid {
		r.ProjectSMTPID = &smtp.Int64
	}
	return r, nil
}

type ProjectHealth struct {
	ProjectID           int64
	ConsecutiveFailures int
	LastStatus          string
	UpdatedAt           time.Time
}

func (s *Store) GetProjectHealth(ctx context.Context, projectID int64) (ProjectHealth, error) {
	_ = s.ensureProjectHealth(ctx, projectID)
	var h ProjectHealth
	err := s.pool.QueryRow(ctx, `
		SELECT project_id, consecutive_failures, last_status, updated_at
		FROM project_health
		WHERE project_id=$1
	`, projectID).Scan(&h.ProjectID, &h.ConsecutiveFailures, &h.LastStatus, &h.UpdatedAt)
	if err != nil {
		return ProjectHealth{}, err
	}
	return h, nil
}

func (s *Store) ensureProjectHealth(ctx context.Context, projectID int64) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO project_health(project_id) VALUES ($1) ON CONFLICT DO NOTHING`, projectID)
	return err
}

func (s *Store) SetProjectHealth(ctx context.Context, projectID int64, consecutiveFailures int, lastStatus string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO project_health(project_id, consecutive_failures, last_status, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT(project_id)
		DO UPDATE SET consecutive_failures = EXCLUDED.consecutive_failures,
		              last_status = EXCLUDED.last_status,
		              updated_at = NOW()
	`, projectID, consecutiveFailures, lastStatus)
	return err
}

type Incident struct {
	ID              int64      `json:"id"`
	ProjectID       int64      `json:"project_id"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	ErrorMessage    string     `json:"error_message"`
	LastAlertSentAt *time.Time `json:"last_alert_sent_at,omitempty"`
}

type LogEntry struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type CheckRun struct {
	ID             int64     `json:"id"`
	CheckID        int64     `json:"check_id"`
	ProjectID      int64     `json:"project_id"`
	Status         string    `json:"status"`
	ResponseTimeMs *int      `json:"response_time_ms,omitempty"`
	ErrorMessage   *string   `json:"error_message,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Store) GetOpenIncident(ctx context.Context, projectID int64) (*Incident, error) {
	query := `
		SELECT id, project_id, status, started_at, resolved_at, error_message, last_alert_sent_at
		FROM incidents
		WHERE project_id=$1 AND status='open'
		LIMIT 1
	`
	var inc Incident
	var resolved sql.NullTime
	var lastAlert sql.NullTime
	err := s.pool.QueryRow(ctx, query, projectID).Scan(
		&inc.ID,
		&inc.ProjectID,
		&inc.Status,
		&inc.StartedAt,
		&resolved,
		&inc.ErrorMessage,
		&lastAlert,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if resolved.Valid {
		v := resolved.Time
		inc.ResolvedAt = &v
	}
	if lastAlert.Valid {
		v := lastAlert.Time
		inc.LastAlertSentAt = &v
	}
	return &inc, nil
}

func (s *Store) CreateIncident(ctx context.Context, projectID int64, errorMessage string) (Incident, error) {
	query := `
		INSERT INTO incidents(project_id, status, error_message)
		VALUES ($1, 'open', $2)
		ON CONFLICT (project_id) WHERE status='open' DO NOTHING
		RETURNING id, project_id, status, started_at, resolved_at, error_message, last_alert_sent_at
	`
	var inc Incident
	var resolved sql.NullTime
	var lastAlert sql.NullTime
	err := s.pool.QueryRow(ctx, query, projectID, truncate(errorMessage, 1024)).Scan(
		&inc.ID,
		&inc.ProjectID,
		&inc.Status,
		&inc.StartedAt,
		&resolved,
		&inc.ErrorMessage,
		&lastAlert,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			existing, getErr := s.GetOpenIncident(ctx, projectID)
			if getErr != nil {
				return Incident{}, getErr
			}
			if existing == nil {
				return Incident{}, fmt.Errorf("failed to create incident for project %d", projectID)
			}
			return *existing, nil
		}
		return Incident{}, err
	}
	if resolved.Valid {
		v := resolved.Time
		inc.ResolvedAt = &v
	}
	if lastAlert.Valid {
		v := lastAlert.Time
		inc.LastAlertSentAt = &v
	}
	return inc, nil
}

func (s *Store) ResolveIncident(ctx context.Context, incidentID int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE incidents
		SET status='resolved', resolved_at=NOW()
		WHERE id=$1 AND status='open'
	`, incidentID)
	return err
}

func (s *Store) UpdateIncidentAlertTime(ctx context.Context, incidentID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE incidents SET last_alert_sent_at=NOW() WHERE id=$1`, incidentID)
	return err
}

func (s *Store) InsertCheckRun(ctx context.Context, checkID int64, projectID int64, status string, responseTimeMs int, errorMessage string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO check_runs(check_id, project_id, status, response_time_ms, error_message)
		VALUES ($1, $2, $3, $4, $5)
	`, checkID, projectID, status, nullableInt(responseTimeMs), nullableString(errorMessage))
	return err
}

func (s *Store) InsertLog(ctx context.Context, projectID int64, level, message string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO logs(project_id, level, message, timestamp)
		VALUES ($1, $2, $3, NOW())
	`, projectID, level, truncate(message, 2048))
	return err
}

func (s *Store) ListLogsByProject(ctx context.Context, projectID int64, limit int) ([]LogEntry, error) {
	limit = clampLimit(limit, 100)
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, level, message, timestamp
		FROM logs
		WHERE project_id=$1
		ORDER BY timestamp DESC
		LIMIT $2
	`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]LogEntry, 0, limit)
	for rows.Next() {
		var item LogEntry
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Level, &item.Message, &item.Timestamp); err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

func (s *Store) ListIncidentsByProject(ctx context.Context, projectID int64, limit int) ([]Incident, error) {
	limit = clampLimit(limit, 50)
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, status, started_at, resolved_at, error_message, last_alert_sent_at
		FROM incidents
		WHERE project_id=$1
		ORDER BY started_at DESC
		LIMIT $2
	`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]Incident, 0, limit)
	for rows.Next() {
		var inc Incident
		var resolved sql.NullTime
		var alert sql.NullTime
		if err := rows.Scan(&inc.ID, &inc.ProjectID, &inc.Status, &inc.StartedAt, &resolved, &inc.ErrorMessage, &alert); err != nil {
			return nil, err
		}
		if resolved.Valid {
			t := resolved.Time
			inc.ResolvedAt = &t
		}
		if alert.Valid {
			t := alert.Time
			inc.LastAlertSentAt = &t
		}
		res = append(res, inc)
	}
	return res, rows.Err()
}

func (s *Store) ListCheckRunsByProject(ctx context.Context, projectID int64, limit int) ([]CheckRun, error) {
	limit = clampLimit(limit, 100)
	rows, err := s.pool.Query(ctx, `
		SELECT id, check_id, project_id, status, response_time_ms, error_message, created_at
		FROM check_runs
		WHERE project_id=$1
		ORDER BY created_at DESC
		LIMIT $2
	`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]CheckRun, 0, limit)
	for rows.Next() {
		var item CheckRun
		var response sql.NullInt32
		var errMessage sql.NullString
		if err := rows.Scan(&item.ID, &item.CheckID, &item.ProjectID, &item.Status, &response, &errMessage, &item.CreatedAt); err != nil {
			return nil, err
		}
		if response.Valid {
			v := int(response.Int32)
			item.ResponseTimeMs = &v
		}
		if errMessage.Valid {
			v := errMessage.String
			item.ErrorMessage = &v
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

type Fix struct {
	ID                    int64  `json:"id"`
	Name                  string `json:"name"`
	Type                  string `json:"type"`
	ScriptPath            string `json:"script_path"`
	SupportedErrorPattern string `json:"supported_error_pattern"`
	TimeoutSec            int    `json:"timeout_sec"`
}

type CreateFixParams struct {
	Name                  string `json:"name"`
	Type                  string `json:"type"`
	ScriptPath            string `json:"script_path"`
	SupportedErrorPattern string `json:"supported_error_pattern"`
	TimeoutSec            int    `json:"timeout_sec"`
}

func (s *Store) FindMatchingFix(ctx context.Context, projectID int64, checkType, errMessage string) (*Fix, error) {
	query := `
		SELECT f.id, f.name, f.type, f.script_path, f.supported_error_pattern, f.timeout_sec
		FROM fixes f
		JOIN project_fixes pf ON pf.fix_id = f.id
		WHERE pf.project_id = $1
		  AND (f.type = $2 OR f.type = 'any')
		ORDER BY f.id ASC
	`
	rows, err := s.pool.Query(ctx, query, projectID, checkType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var f Fix
		if err := rows.Scan(&f.ID, &f.Name, &f.Type, &f.ScriptPath, &f.SupportedErrorPattern, &f.TimeoutSec); err != nil {
			return nil, err
		}
		matched, matchErr := regexp.MatchString(f.SupportedErrorPattern, errMessage)
		if matchErr != nil {
			continue
		}
		if matched {
			return &f, nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *Store) ListProjectFixes(ctx context.Context, projectID int64) ([]Fix, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT f.id, f.name, f.type, f.script_path, f.supported_error_pattern, f.timeout_sec
		FROM fixes f
		JOIN project_fixes pf ON pf.fix_id = f.id
		WHERE pf.project_id=$1
		ORDER BY f.id ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]Fix, 0)
	for rows.Next() {
		var f Fix
		if err := rows.Scan(&f.ID, &f.Name, &f.Type, &f.ScriptPath, &f.SupportedErrorPattern, &f.TimeoutSec); err != nil {
			return nil, err
		}
		res = append(res, f)
	}
	return res, rows.Err()
}

func (s *Store) GetProjectFix(ctx context.Context, projectID, fixID int64) (*Fix, error) {
	var f Fix
	err := s.pool.QueryRow(ctx, `
		SELECT f.id, f.name, f.type, f.script_path, f.supported_error_pattern, f.timeout_sec
		FROM fixes f
		JOIN project_fixes pf ON pf.fix_id = f.id
		WHERE pf.project_id=$1 AND pf.fix_id=$2
	`, projectID, fixID).Scan(&f.ID, &f.Name, &f.Type, &f.ScriptPath, &f.SupportedErrorPattern, &f.TimeoutSec)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

func (s *Store) CreateFix(ctx context.Context, p CreateFixParams) (Fix, error) {
	if p.Type == "" {
		p.Type = "any"
	}
	if p.TimeoutSec <= 0 {
		p.TimeoutSec = 30
	}
	var f Fix
	err := s.pool.QueryRow(ctx, `
		INSERT INTO fixes(name, type, script_path, supported_error_pattern, timeout_sec)
		VALUES($1, $2, $3, $4, $5)
		RETURNING id, name, type, script_path, supported_error_pattern, timeout_sec
	`, strings.TrimSpace(p.Name), strings.TrimSpace(p.Type), strings.TrimSpace(p.ScriptPath), strings.TrimSpace(p.SupportedErrorPattern), p.TimeoutSec).
		Scan(&f.ID, &f.Name, &f.Type, &f.ScriptPath, &f.SupportedErrorPattern, &f.TimeoutSec)
	if err != nil {
		return Fix{}, err
	}
	return f, nil
}

func (s *Store) AttachFixToProject(ctx context.Context, projectID, fixID int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO project_fixes(project_id, fix_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, projectID, fixID)
	return err
}

type SMTPProfile struct {
	ID                int64
	Host              string
	Port              int
	Username          string
	PasswordEncrypted string
	FromEmail         string
}

func (s *Store) GetSMTPProfile(ctx context.Context, id int64) (*SMTPProfile, error) {
	var p SMTPProfile
	err := s.pool.QueryRow(ctx, `
		SELECT id, host, port, username, password_encrypted, from_email
		FROM smtp_profiles
		WHERE id=$1
	`, id).Scan(&p.ID, &p.Host, &p.Port, &p.Username, &p.PasswordEncrypted, &p.FromEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) CreateSMTPProfile(ctx context.Context, host string, port int, username, encryptedPassword, fromEmail string) (SMTPProfile, error) {
	var p SMTPProfile
	err := s.pool.QueryRow(ctx, `
		INSERT INTO smtp_profiles(host, port, username, password_encrypted, from_email)
		VALUES($1, $2, $3, $4, $5)
		RETURNING id, host, port, username, password_encrypted, from_email
	`, host, port, username, encryptedPassword, fromEmail).
		Scan(&p.ID, &p.Host, &p.Port, &p.Username, &p.PasswordEncrypted, &p.FromEmail)
	if err != nil {
		return SMTPProfile{}, err
	}
	return p, nil
}

func nullInt64Arg(v sql.NullInt64) interface{} {
	if v.Valid {
		return v.Int64
	}
	return nil
}

func nullIntArg(v *int) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt(v int) interface{} {
	if v <= 0 {
		return nil
	}
	return v
}

func nullableString(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return truncate(s, 2048)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func clampLimit(limit, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > 500 {
		return 500
	}
	return limit
}
