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
	ID                       int64     `json:"id"`
	Name                     string    `json:"name"`
	Domain                   string    `json:"domain"`
	CheckIntervalSec         int       `json:"check_interval_sec"`
	FailureThreshold         int       `json:"failure_threshold"`
	AutofixEnabled           bool      `json:"autofix_enabled"`
	MaxAutofixRetries        int       `json:"max_autofix_retries"`
	SMTPProfileID            *int64    `json:"smtp_profile_id,omitempty"`
	AlertEmails              []string  `json:"alert_emails"`
	EmailSubjectOpened       string    `json:"email_subject_opened"`
	EmailBodyOpened          string    `json:"email_body_opened"`
	EmailSubjectResolved     string    `json:"email_subject_resolved"`
	EmailBodyResolved        string    `json:"email_body_resolved"`
	EmailSubjectRepeated     string    `json:"email_subject_repeated"`
	EmailBodyRepeated        string    `json:"email_body_repeated"`
	EmailSubjectAutofixLimit string    `json:"email_subject_autofix_limit"`
	EmailBodyAutofixLimit    string    `json:"email_body_autofix_limit"`
	NextCheckAt              time.Time `json:"next_check_at"`
	CreatedAt                time.Time `json:"created_at"`
}

type CreateProjectParams struct {
	Name                     string   `json:"name"`
	Domain                   string   `json:"domain"`
	CheckIntervalSec         int      `json:"check_interval_sec"`
	FailureThreshold         int      `json:"failure_threshold"`
	AutofixEnabled           bool     `json:"autofix_enabled"`
	MaxAutofixRetries        int      `json:"max_autofix_retries"`
	SMTPProfileID            *int64   `json:"smtp_profile_id"`
	AlertEmails              []string `json:"alert_emails"`
	EmailSubjectOpened       string   `json:"email_subject_opened"`
	EmailBodyOpened          string   `json:"email_body_opened"`
	EmailSubjectResolved     string   `json:"email_subject_resolved"`
	EmailBodyResolved        string   `json:"email_body_resolved"`
	EmailSubjectRepeated     string   `json:"email_subject_repeated"`
	EmailBodyRepeated        string   `json:"email_body_repeated"`
	EmailSubjectAutofixLimit string   `json:"email_subject_autofix_limit"`
	EmailBodyAutofixLimit    string   `json:"email_body_autofix_limit"`
}

type UpdateProjectParams struct {
	Name                     string   `json:"name"`
	Domain                   string   `json:"domain"`
	CheckIntervalSec         int      `json:"check_interval_sec"`
	FailureThreshold         int      `json:"failure_threshold"`
	AutofixEnabled           bool     `json:"autofix_enabled"`
	MaxAutofixRetries        int      `json:"max_autofix_retries"`
	SMTPProfileID            *int64   `json:"smtp_profile_id"`
	AlertEmails              []string `json:"alert_emails"`
	EmailSubjectOpened       string   `json:"email_subject_opened"`
	EmailBodyOpened          string   `json:"email_body_opened"`
	EmailSubjectResolved     string   `json:"email_subject_resolved"`
	EmailBodyResolved        string   `json:"email_body_resolved"`
	EmailSubjectRepeated     string   `json:"email_subject_repeated"`
	EmailBodyRepeated        string   `json:"email_body_repeated"`
	EmailSubjectAutofixLimit string   `json:"email_subject_autofix_limit"`
	EmailBodyAutofixLimit    string   `json:"email_body_autofix_limit"`
}

const (
	defaultEmailSubjectOpened       = "[DOWN] {domain} is unreachable"
	defaultEmailBodyOpened          = "Project: {project_name}\nDomain: {domain}\nEvent: opened\nIncident ID: {incident_id}\nCheck: #{check_id} {check_type} {check_target}\nError: {error}\nTimestamp: {timestamp}\nAutofix: {autofix_status}"
	defaultEmailSubjectResolved     = "[RESOLVED] {domain} recovered"
	defaultEmailBodyResolved        = "Project: {project_name}\nDomain: {domain}\nEvent: resolved\nIncident ID: {incident_id}\nCheck: #{check_id} {check_type} {check_target}\nTimestamp: {timestamp}\nAutofix: {autofix_status}"
	defaultEmailSubjectRepeated     = "[DOWN][REPEATED] {domain} still failing"
	defaultEmailBodyRepeated        = "Project: {project_name}\nDomain: {domain}\nEvent: repeated\nIncident ID: {incident_id}\nCheck: #{check_id} {check_type} {check_target}\nError: {error}\nTimestamp: {timestamp}\nAutofix: {autofix_status}"
	defaultEmailSubjectAutofixLimit = "[AUTOFIX LIMIT] {domain} retries exhausted"
	defaultEmailBodyAutofixLimit    = "Project: {project_name}\nDomain: {domain}\nIncident ID: {incident_id}\nAutofix attempts: {autofix_attempts}\nMax retries: {max_retries}\nTimestamp: {timestamp}\n\nAutomatic fixes have been exhausted. Manual intervention required."
)

func normalizeProjectEmailTemplates(p *Project) {
	if strings.TrimSpace(p.EmailSubjectOpened) == "" {
		p.EmailSubjectOpened = defaultEmailSubjectOpened
	}
	if strings.TrimSpace(p.EmailBodyOpened) == "" {
		p.EmailBodyOpened = defaultEmailBodyOpened
	}
	if strings.TrimSpace(p.EmailSubjectResolved) == "" {
		p.EmailSubjectResolved = defaultEmailSubjectResolved
	}
	if strings.TrimSpace(p.EmailBodyResolved) == "" {
		p.EmailBodyResolved = defaultEmailBodyResolved
	}
	if strings.TrimSpace(p.EmailSubjectRepeated) == "" {
		p.EmailSubjectRepeated = defaultEmailSubjectRepeated
	}
	if strings.TrimSpace(p.EmailBodyRepeated) == "" {
		p.EmailBodyRepeated = defaultEmailBodyRepeated
	}
	if strings.TrimSpace(p.EmailSubjectAutofixLimit) == "" {
		p.EmailSubjectAutofixLimit = defaultEmailSubjectAutofixLimit
	}
	if strings.TrimSpace(p.EmailBodyAutofixLimit) == "" {
		p.EmailBodyAutofixLimit = defaultEmailBodyAutofixLimit
	}
}

func normalizeCreateProjectEmailParams(p *CreateProjectParams) {
	if strings.TrimSpace(p.EmailSubjectOpened) == "" {
		p.EmailSubjectOpened = defaultEmailSubjectOpened
	}
	if strings.TrimSpace(p.EmailBodyOpened) == "" {
		p.EmailBodyOpened = defaultEmailBodyOpened
	}
	if strings.TrimSpace(p.EmailSubjectResolved) == "" {
		p.EmailSubjectResolved = defaultEmailSubjectResolved
	}
	if strings.TrimSpace(p.EmailBodyResolved) == "" {
		p.EmailBodyResolved = defaultEmailBodyResolved
	}
	if strings.TrimSpace(p.EmailSubjectRepeated) == "" {
		p.EmailSubjectRepeated = defaultEmailSubjectRepeated
	}
	if strings.TrimSpace(p.EmailBodyRepeated) == "" {
		p.EmailBodyRepeated = defaultEmailBodyRepeated
	}
	if strings.TrimSpace(p.EmailSubjectAutofixLimit) == "" {
		p.EmailSubjectAutofixLimit = defaultEmailSubjectAutofixLimit
	}
	if strings.TrimSpace(p.EmailBodyAutofixLimit) == "" {
		p.EmailBodyAutofixLimit = defaultEmailBodyAutofixLimit
	}
}

func normalizeProjectEmailParams(p *UpdateProjectParams) {
	if strings.TrimSpace(p.EmailSubjectOpened) == "" {
		p.EmailSubjectOpened = defaultEmailSubjectOpened
	}
	if strings.TrimSpace(p.EmailBodyOpened) == "" {
		p.EmailBodyOpened = defaultEmailBodyOpened
	}
	if strings.TrimSpace(p.EmailSubjectResolved) == "" {
		p.EmailSubjectResolved = defaultEmailSubjectResolved
	}
	if strings.TrimSpace(p.EmailBodyResolved) == "" {
		p.EmailBodyResolved = defaultEmailBodyResolved
	}
	if strings.TrimSpace(p.EmailSubjectRepeated) == "" {
		p.EmailSubjectRepeated = defaultEmailSubjectRepeated
	}
	if strings.TrimSpace(p.EmailBodyRepeated) == "" {
		p.EmailBodyRepeated = defaultEmailBodyRepeated
	}
	if strings.TrimSpace(p.EmailSubjectAutofixLimit) == "" {
		p.EmailSubjectAutofixLimit = defaultEmailSubjectAutofixLimit
	}
	if strings.TrimSpace(p.EmailBodyAutofixLimit) == "" {
		p.EmailBodyAutofixLimit = defaultEmailBodyAutofixLimit
	}
}

func (s *Store) CreateProject(ctx context.Context, p CreateProjectParams) (Project, error) {
	if p.FailureThreshold <= 0 {
		p.FailureThreshold = 3
	}
	if p.MaxAutofixRetries <= 0 {
		p.MaxAutofixRetries = 3
	}
	if p.AlertEmails == nil {
		p.AlertEmails = []string{}
	}
	normalizeCreateProjectEmailParams(&p)
	var project Project
	var smtp sql.NullInt64
	if p.SMTPProfileID != nil {
		smtp = sql.NullInt64{Int64: *p.SMTPProfileID, Valid: true}
	}
	query := `
		INSERT INTO projects (
			name, domain, check_interval_sec, failure_threshold, autofix_enabled, max_autofix_retries, smtp_profile_id, alert_emails,
			email_subject_opened, email_body_opened, email_subject_resolved, email_body_resolved,
			email_subject_repeated, email_body_repeated, email_subject_autofix_limit, email_body_autofix_limit
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING
			id, name, domain, check_interval_sec, failure_threshold, autofix_enabled, max_autofix_retries, smtp_profile_id, alert_emails,
			email_subject_opened, email_body_opened, email_subject_resolved, email_body_resolved,
			email_subject_repeated, email_body_repeated, email_subject_autofix_limit, email_body_autofix_limit,
			next_check_at, created_at
	`
	var smtpID sql.NullInt64
	err := s.pool.QueryRow(ctx, query,
		strings.TrimSpace(p.Name),
		strings.TrimSpace(p.Domain),
		p.CheckIntervalSec,
		p.FailureThreshold,
		p.AutofixEnabled,
		p.MaxAutofixRetries,
		nullInt64Arg(smtp),
		p.AlertEmails,
		p.EmailSubjectOpened,
		p.EmailBodyOpened,
		p.EmailSubjectResolved,
		p.EmailBodyResolved,
		p.EmailSubjectRepeated,
		p.EmailBodyRepeated,
		p.EmailSubjectAutofixLimit,
		p.EmailBodyAutofixLimit,
	).Scan(
		&project.ID,
		&project.Name,
		&project.Domain,
		&project.CheckIntervalSec,
		&project.FailureThreshold,
		&project.AutofixEnabled,
		&project.MaxAutofixRetries,
		&smtpID,
		&project.AlertEmails,
		&project.EmailSubjectOpened,
		&project.EmailBodyOpened,
		&project.EmailSubjectResolved,
		&project.EmailBodyResolved,
		&project.EmailSubjectRepeated,
		&project.EmailBodyRepeated,
		&project.EmailSubjectAutofixLimit,
		&project.EmailBodyAutofixLimit,
		&project.NextCheckAt,
		&project.CreatedAt,
	)
	if err != nil {
		return Project{}, err
	}
	if smtpID.Valid {
		project.SMTPProfileID = &smtpID.Int64
	}
	normalizeProjectEmailTemplates(&project)
	_, err = s.pool.Exec(ctx, `INSERT INTO project_health(project_id) VALUES($1) ON CONFLICT DO NOTHING`, project.ID)
	if err != nil {
		return Project{}, err
	}
	return project, nil
}

func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	query := `
		SELECT
			id, name, domain, check_interval_sec, failure_threshold, autofix_enabled, max_autofix_retries, smtp_profile_id, alert_emails,
			email_subject_opened, email_body_opened, email_subject_resolved, email_body_resolved,
			email_subject_repeated, email_body_repeated, email_subject_autofix_limit, email_body_autofix_limit,
			next_check_at, created_at
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
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Domain, &p.CheckIntervalSec, &p.FailureThreshold, &p.AutofixEnabled, &p.MaxAutofixRetries, &smtpID, &p.AlertEmails,
			&p.EmailSubjectOpened, &p.EmailBodyOpened, &p.EmailSubjectResolved, &p.EmailBodyResolved,
			&p.EmailSubjectRepeated, &p.EmailBodyRepeated, &p.EmailSubjectAutofixLimit, &p.EmailBodyAutofixLimit,
			&p.NextCheckAt, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		if smtpID.Valid {
			p.SMTPProfileID = &smtpID.Int64
		}
		normalizeProjectEmailTemplates(&p)
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *Store) GetProjectByID(ctx context.Context, projectID int64) (*Project, error) {
	var project Project
	var smtpID sql.NullInt64
	err := s.pool.QueryRow(ctx, `
		SELECT
			id, name, domain, check_interval_sec, failure_threshold, autofix_enabled, max_autofix_retries, smtp_profile_id, alert_emails,
			email_subject_opened, email_body_opened, email_subject_resolved, email_body_resolved,
			email_subject_repeated, email_body_repeated, email_subject_autofix_limit, email_body_autofix_limit,
			next_check_at, created_at
		FROM projects
		WHERE id=$1
	`, projectID).Scan(
		&project.ID,
		&project.Name,
		&project.Domain,
		&project.CheckIntervalSec,
		&project.FailureThreshold,
		&project.AutofixEnabled,
		&project.MaxAutofixRetries,
		&smtpID,
		&project.AlertEmails,
		&project.EmailSubjectOpened,
		&project.EmailBodyOpened,
		&project.EmailSubjectResolved,
		&project.EmailBodyResolved,
		&project.EmailSubjectRepeated,
		&project.EmailBodyRepeated,
		&project.EmailSubjectAutofixLimit,
		&project.EmailBodyAutofixLimit,
		&project.NextCheckAt,
		&project.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if smtpID.Valid {
		project.SMTPProfileID = &smtpID.Int64
	}
	normalizeProjectEmailTemplates(&project)
	return &project, nil
}

func (s *Store) UpdateProject(ctx context.Context, projectID int64, p UpdateProjectParams) (Project, error) {
	if p.CheckIntervalSec <= 0 {
		return Project{}, errors.New("check interval must be greater than 0")
	}
	if p.FailureThreshold <= 0 {
		return Project{}, errors.New("failure threshold must be greater than 0")
	}
	if p.MaxAutofixRetries < 0 {
		p.MaxAutofixRetries = 3
	}
	if p.AlertEmails == nil {
		p.AlertEmails = []string{}
	}
	normalizeProjectEmailParams(&p)

	var project Project
	var smtpArg sql.NullInt64
	if p.SMTPProfileID != nil {
		smtpArg = sql.NullInt64{Int64: *p.SMTPProfileID, Valid: true}
	}
	var smtpID sql.NullInt64
	err := s.pool.QueryRow(ctx, `
		UPDATE projects
		SET
			name=$2,
			domain=$3,
			check_interval_sec=$4,
			failure_threshold=$5,
			autofix_enabled=$6,
			max_autofix_retries=$7,
			smtp_profile_id=$8,
			alert_emails=$9,
			email_subject_opened=$10,
			email_body_opened=$11,
			email_subject_resolved=$12,
			email_body_resolved=$13,
			email_subject_repeated=$14,
			email_body_repeated=$15,
			email_subject_autofix_limit=$16,
			email_body_autofix_limit=$17
		WHERE id=$1
		RETURNING
			id, name, domain, check_interval_sec, failure_threshold, autofix_enabled, max_autofix_retries, smtp_profile_id, alert_emails,
			email_subject_opened, email_body_opened, email_subject_resolved, email_body_resolved,
			email_subject_repeated, email_body_repeated, email_subject_autofix_limit, email_body_autofix_limit,
			next_check_at, created_at
	`,
		projectID,
		strings.TrimSpace(p.Name),
		strings.TrimSpace(p.Domain),
		p.CheckIntervalSec,
		p.FailureThreshold,
		p.AutofixEnabled,
		p.MaxAutofixRetries,
		nullInt64Arg(smtpArg),
		p.AlertEmails,
		p.EmailSubjectOpened,
		p.EmailBodyOpened,
		p.EmailSubjectResolved,
		p.EmailBodyResolved,
		p.EmailSubjectRepeated,
		p.EmailBodyRepeated,
		p.EmailSubjectAutofixLimit,
		p.EmailBodyAutofixLimit,
	).Scan(
		&project.ID,
		&project.Name,
		&project.Domain,
		&project.CheckIntervalSec,
		&project.FailureThreshold,
		&project.AutofixEnabled,
		&project.MaxAutofixRetries,
		&smtpID,
		&project.AlertEmails,
		&project.EmailSubjectOpened,
		&project.EmailBodyOpened,
		&project.EmailSubjectResolved,
		&project.EmailBodyResolved,
		&project.EmailSubjectRepeated,
		&project.EmailBodyRepeated,
		&project.EmailSubjectAutofixLimit,
		&project.EmailBodyAutofixLimit,
		&project.NextCheckAt,
		&project.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Project{}, fmt.Errorf("project %d not found", projectID)
		}
		return Project{}, err
	}
	if smtpID.Valid {
		project.SMTPProfileID = &smtpID.Int64
	}
	normalizeProjectEmailTemplates(&project)
	return project, nil
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

func (s *Store) DeleteProject(ctx context.Context, projectID int64) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
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

type ReplaceCheckParams struct {
	ID             *int64 `json:"id"`
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

func (s *Store) ReplaceProjectChecks(ctx context.Context, projectID int64, checks []ReplaceCheckParams) ([]Check, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var projectExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1)`, projectID).Scan(&projectExists); err != nil {
		return nil, err
	}
	if !projectExists {
		return nil, fmt.Errorf("project %d not found", projectID)
	}

	existingRows, err := tx.Query(ctx, `SELECT id FROM checks WHERE project_id=$1`, projectID)
	if err != nil {
		return nil, err
	}
	existingIDs := make(map[int64]struct{})
	for existingRows.Next() {
		var id int64
		if err := existingRows.Scan(&id); err != nil {
			existingRows.Close()
			return nil, err
		}
		existingIDs[id] = struct{}{}
	}
	if err := existingRows.Err(); err != nil {
		existingRows.Close()
		return nil, err
	}
	existingRows.Close()

	seenInputIDs := make(map[int64]struct{})
	keepIDs := make([]int64, 0, len(checks))
	for _, in := range checks {
		checkType := strings.ToLower(strings.TrimSpace(in.Type))
		if checkType != "http" && checkType != "tcp" && checkType != "ping" {
			return nil, fmt.Errorf("unsupported check type: %s", in.Type)
		}
		target := strings.TrimSpace(in.Target)
		if target == "" {
			return nil, errors.New("check target is required")
		}

		timeout := in.TimeoutMs
		if timeout <= 0 {
			timeout = 5000
		}

		if in.ID != nil && *in.ID > 0 {
			checkID := *in.ID
			if _, dup := seenInputIDs[checkID]; dup {
				return nil, fmt.Errorf("duplicate check id in request: %d", checkID)
			}
			seenInputIDs[checkID] = struct{}{}
			if _, ok := existingIDs[checkID]; !ok {
				return nil, fmt.Errorf("check %d does not belong to project %d", checkID, projectID)
			}
			cmd, err := tx.Exec(ctx, `
				UPDATE checks
				SET type=$3, target=$4, timeout_ms=$5, expected_status=$6
				WHERE id=$1 AND project_id=$2
			`, checkID, projectID, checkType, target, timeout, nullIntArg(in.ExpectedStatus))
			if err != nil {
				return nil, err
			}
			if cmd.RowsAffected() == 0 {
				return nil, fmt.Errorf("check %d does not belong to project %d", checkID, projectID)
			}
			keepIDs = append(keepIDs, checkID)
			continue
		}

		var newID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO checks(project_id, type, target, timeout_ms, expected_status)
			VALUES($1, $2, $3, $4, $5)
			RETURNING id
		`, projectID, checkType, target, timeout, nullIntArg(in.ExpectedStatus)).Scan(&newID); err != nil {
			return nil, err
		}
		keepIDs = append(keepIDs, newID)
	}

	if len(keepIDs) == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM checks WHERE project_id=$1`, projectID); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.Exec(ctx, `DELETE FROM checks WHERE project_id=$1 AND NOT (id = ANY($2))`, projectID, keepIDs); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.ListChecksByProject(ctx, projectID)
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
	ProjectName              string
	ProjectDomain            string
	FailureThreshold         int
	AutofixEnabled           bool
	MaxAutofixRetries        int
	ProjectSMTPID            *int64
	AlertEmails              []string
	EmailSubjectOpened       string
	EmailBodyOpened          string
	EmailSubjectResolved     string
	EmailBodyResolved        string
	EmailSubjectRepeated     string
	EmailBodyRepeated        string
	EmailSubjectAutofixLimit string
	EmailBodyAutofixLimit    string
	CheckIntervalSec         int
	ProjectNextCheck         time.Time
	ProjectCreatedAt         time.Time
}

func (s *Store) GetCheckContext(ctx context.Context, checkID int64) (CheckContext, error) {
	query := `
		SELECT c.id, c.project_id, c.type, c.target, c.timeout_ms, c.expected_status, c.created_at,
		       p.name, p.domain, p.failure_threshold, p.autofix_enabled, p.max_autofix_retries, p.smtp_profile_id, p.alert_emails,
		       p.email_subject_opened, p.email_body_opened, p.email_subject_resolved, p.email_body_resolved,
		       p.email_subject_repeated, p.email_body_repeated, p.email_subject_autofix_limit, p.email_body_autofix_limit,
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
		&r.MaxAutofixRetries,
		&smtp,
		&r.AlertEmails,
		&r.EmailSubjectOpened,
		&r.EmailBodyOpened,
		&r.EmailSubjectResolved,
		&r.EmailBodyResolved,
		&r.EmailSubjectRepeated,
		&r.EmailBodyRepeated,
		&r.EmailSubjectAutofixLimit,
		&r.EmailBodyAutofixLimit,
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
	normalizeCheckContextTemplates(&r)
	return r, nil
}

func normalizeCheckContextTemplates(c *CheckContext) {
	if strings.TrimSpace(c.EmailSubjectOpened) == "" {
		c.EmailSubjectOpened = defaultEmailSubjectOpened
	}
	if strings.TrimSpace(c.EmailBodyOpened) == "" {
		c.EmailBodyOpened = defaultEmailBodyOpened
	}
	if strings.TrimSpace(c.EmailSubjectResolved) == "" {
		c.EmailSubjectResolved = defaultEmailSubjectResolved
	}
	if strings.TrimSpace(c.EmailBodyResolved) == "" {
		c.EmailBodyResolved = defaultEmailBodyResolved
	}
	if strings.TrimSpace(c.EmailSubjectRepeated) == "" {
		c.EmailSubjectRepeated = defaultEmailSubjectRepeated
	}
	if strings.TrimSpace(c.EmailBodyRepeated) == "" {
		c.EmailBodyRepeated = defaultEmailBodyRepeated
	}
	if strings.TrimSpace(c.EmailSubjectAutofixLimit) == "" {
		c.EmailSubjectAutofixLimit = defaultEmailSubjectAutofixLimit
	}
	if strings.TrimSpace(c.EmailBodyAutofixLimit) == "" {
		c.EmailBodyAutofixLimit = defaultEmailBodyAutofixLimit
	}
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
	AutofixAttempts int        `json:"autofix_attempts"`
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

type PathHealth struct {
	CheckID            int64      `json:"check_id"`
	Type               string     `json:"type"`
	Target             string     `json:"target"`
	TimeoutMs          int        `json:"timeout_ms"`
	ExpectedStatus     *int       `json:"expected_status,omitempty"`
	LastStatus         string     `json:"last_status"`
	LastCheckedAt      *time.Time `json:"last_checked_at,omitempty"`
	LastResponseTimeMs *int       `json:"last_response_time_ms,omitempty"`
	LastErrorMessage   *string    `json:"last_error_message,omitempty"`
	Runs1h             int        `json:"runs_1h"`
	Healthy1h          int        `json:"healthy_1h"`
	Failed1h           int        `json:"failed_1h"`
	SuccessRate1h      float64    `json:"success_rate_1h"`
}

type UptimePoint struct {
	Start          time.Time `json:"start"`
	End            time.Time `json:"end"`
	UpSeconds      int       `json:"up_seconds"`
	DownSeconds    int       `json:"down_seconds"`
	UnknownSeconds int       `json:"unknown_seconds"`
	UptimeRatio    float64   `json:"uptime_ratio"`
}

type UptimeState struct {
	CurrentStatus string    `json:"current_status"`
	CursorAt      time.Time `json:"cursor_at"`
}

type uptimeAgg struct {
	up   int
	down int
}

func (s *Store) GetOpenIncident(ctx context.Context, projectID int64) (*Incident, error) {
	query := `
		SELECT id, project_id, status, started_at, resolved_at, error_message, last_alert_sent_at, autofix_attempts
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
		&inc.AutofixAttempts,
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
		RETURNING id, project_id, status, started_at, resolved_at, error_message, last_alert_sent_at, autofix_attempts
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
		&inc.AutofixAttempts,
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

func (s *Store) IncrementIncidentAutofixAttempts(ctx context.Context, incidentID int64) (int, error) {
	var attempts int
	err := s.pool.QueryRow(ctx, `
		UPDATE incidents SET autofix_attempts = autofix_attempts + 1
		WHERE id=$1
		RETURNING autofix_attempts
	`, incidentID).Scan(&attempts)
	if err != nil {
		return 0, err
	}
	return attempts, nil
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

func (s *Store) ListCheckRunsByCheck(ctx context.Context, projectID, checkID int64, limit int) ([]CheckRun, error) {
	limit = clampLimit(limit, 120)
	rows, err := s.pool.Query(ctx, `
		SELECT id, check_id, project_id, status, response_time_ms, error_message, created_at
		FROM check_runs
		WHERE project_id=$1 AND check_id=$2
		ORDER BY created_at DESC
		LIMIT $3
	`, projectID, checkID, limit)
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

func (s *Store) ListPathHealthByProject(ctx context.Context, projectID int64) ([]PathHealth, error) {
	rows, err := s.pool.Query(ctx, `
		WITH latest_runs AS (
			SELECT DISTINCT ON (check_id)
				check_id,
				status,
				response_time_ms,
				error_message,
				created_at
			FROM check_runs
			WHERE project_id=$1
			ORDER BY check_id, created_at DESC
		),
		hourly_stats AS (
			SELECT
				check_id,
				COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '1 hour')::INT AS runs_1h,
				COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '1 hour' AND status='healthy')::INT AS healthy_1h,
				COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '1 hour' AND status='failed')::INT AS failed_1h
			FROM check_runs
			WHERE project_id=$1
			GROUP BY check_id
		)
		SELECT
			c.id,
			c.type,
			c.target,
			c.timeout_ms,
			c.expected_status,
			COALESCE(l.status, 'unknown') AS last_status,
			l.created_at,
			l.response_time_ms,
			l.error_message,
			COALESCE(h.runs_1h, 0),
			COALESCE(h.healthy_1h, 0),
			COALESCE(h.failed_1h, 0)
		FROM checks c
		LEFT JOIN latest_runs l ON l.check_id = c.id
		LEFT JOIN hourly_stats h ON h.check_id = c.id
		WHERE c.project_id=$1
		ORDER BY c.id ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]PathHealth, 0)
	for rows.Next() {
		var item PathHealth
		var expected sql.NullInt32
		var lastChecked sql.NullTime
		var responseTime sql.NullInt32
		var errMessage sql.NullString
		if err := rows.Scan(
			&item.CheckID,
			&item.Type,
			&item.Target,
			&item.TimeoutMs,
			&expected,
			&item.LastStatus,
			&lastChecked,
			&responseTime,
			&errMessage,
			&item.Runs1h,
			&item.Healthy1h,
			&item.Failed1h,
		); err != nil {
			return nil, err
		}
		if expected.Valid {
			v := int(expected.Int32)
			item.ExpectedStatus = &v
		}
		if lastChecked.Valid {
			v := lastChecked.Time
			item.LastCheckedAt = &v
		}
		if responseTime.Valid {
			v := int(responseTime.Int32)
			item.LastResponseTimeMs = &v
		}
		if errMessage.Valid {
			v := errMessage.String
			item.LastErrorMessage = &v
		}
		if item.Runs1h > 0 {
			item.SuccessRate1h = float64(item.Healthy1h) / float64(item.Runs1h)
		}
		res = append(res, item)
	}
	return res, rows.Err()
}

func (s *Store) RecordProjectUptimeStatus(ctx context.Context, projectID int64, status string, at time.Time) error {
	if status != "up" && status != "down" {
		return fmt.Errorf("invalid uptime status: %s", status)
	}
	if at.IsZero() {
		at = time.Now()
	}
	at = at.UTC()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	freshnessWindow, err := projectUptimeFreshnessWindow(ctx, tx, projectID)
	if err != nil {
		return err
	}

	var currentStatus string
	var cursorAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT current_status, cursor_at
		FROM project_uptime_state
		WHERE project_id=$1
		FOR UPDATE
	`, projectID).Scan(&currentStatus, &cursorAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, `
				INSERT INTO project_uptime_state(project_id, current_status, cursor_at, updated_at)
				VALUES($1, $2, $3, NOW())
			`, projectID, status, at)
			if err != nil {
				return err
			}
			return tx.Commit(ctx)
		}
		return err
	}

	if !at.After(cursorAt) {
		_, err = tx.Exec(ctx, `
			UPDATE project_uptime_state
			SET current_status=$2, updated_at=NOW()
			WHERE project_id=$1
		`, projectID, status)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	carryEnd := at
	if at.Sub(cursorAt) > freshnessWindow {
		carryEnd = cursorAt.Add(freshnessWindow)
	}
	if err := accumulateUptimeDurationTx(ctx, tx, projectID, currentStatus, cursorAt, carryEnd); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE project_uptime_state
		SET current_status=$2, cursor_at=$3, updated_at=NOW()
		WHERE project_id=$1
	`, projectID, status, at)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetUptimeSeries(ctx context.Context, projectID int64, from, to time.Time, bucketSize time.Duration) ([]UptimePoint, error) {
	if !to.After(from) {
		return nil, fmt.Errorf("invalid uptime range")
	}
	if bucketSize <= 0 {
		return nil, fmt.Errorf("invalid bucket size")
	}

	from = from.UTC()
	to = to.UTC()
	interval := fmt.Sprintf("%d seconds", int(bucketSize.Seconds()))
	freshnessWindow, err := projectUptimeFreshnessWindow(ctx, s.pool, projectID)
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			date_bin($3::interval, bucket_start, $2::timestamptz) AS slot_start,
			COALESCE(SUM(up_seconds), 0)::INT AS up_seconds,
			COALESCE(SUM(down_seconds), 0)::INT AS down_seconds
		FROM project_uptime_minutes
		WHERE project_id=$1
		  AND bucket_start >= $2
		  AND bucket_start < $4
		GROUP BY slot_start
		ORDER BY slot_start ASC
	`, projectID, from, interval, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bySlot := make(map[time.Time]uptimeAgg)
	for rows.Next() {
		var slotStart time.Time
		var upSeconds int
		var downSeconds int
		if err := rows.Scan(&slotStart, &upSeconds, &downSeconds); err != nil {
			return nil, err
		}
		bySlot[slotStart.UTC()] = uptimeAgg{up: upSeconds, down: downSeconds}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Add tail from last cursor to now for near-real-time uptime.
	state, err := s.getUptimeState(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if state != nil {
		tailStart := state.CursorAt.UTC()
		if tailStart.Before(from) {
			tailStart = from
		}
		tailEnd := time.Now().UTC()
		if tailEnd.After(to) {
			tailEnd = to
		}
		maxTailEnd := state.CursorAt.UTC().Add(freshnessWindow)
		if tailEnd.After(maxTailEnd) {
			tailEnd = maxTailEnd
		}
		if tailEnd.After(tailStart) {
			addTailToSlots(bySlot, from, bucketSize, state.CurrentStatus, tailStart, tailEnd)
		}
	}

	points := make([]UptimePoint, 0)
	for slot := from; slot.Before(to); slot = slot.Add(bucketSize) {
		bucketEnd := slot.Add(bucketSize)
		if bucketEnd.After(to) {
			bucketEnd = to
		}
		durationSec := int(bucketEnd.Sub(slot).Seconds())
		if durationSec <= 0 {
			continue
		}

		val := bySlot[slot]
		up := val.up
		down := val.down
		if up < 0 {
			up = 0
		}
		if down < 0 {
			down = 0
		}
		if up > durationSec {
			up = durationSec
		}
		if down > durationSec {
			down = durationSec
		}

		known := up + down
		if known > durationSec {
			overflow := known - durationSec
			if up >= down {
				up -= overflow
				if up < 0 {
					up = 0
				}
			} else {
				down -= overflow
				if down < 0 {
					down = 0
				}
			}
			known = up + down
		}
		if known > 0 && known < durationSec {
			missing := durationSec - known
			if down > up {
				down += missing
			} else {
				up += missing
			}
		}

		ratio := 0.0
		known = up + down
		if known > 0 {
			ratio = float64(up) / float64(known)
		}

		points = append(points, UptimePoint{
			Start:          slot,
			End:            bucketEnd,
			UpSeconds:      up,
			DownSeconds:    down,
			UnknownSeconds: 0,
			UptimeRatio:    ratio,
		})
	}
	return points, nil
}

func (s *Store) getUptimeState(ctx context.Context, projectID int64) (*UptimeState, error) {
	var state UptimeState
	err := s.pool.QueryRow(ctx, `
		SELECT current_status, cursor_at
		FROM project_uptime_state
		WHERE project_id=$1
	`, projectID).Scan(&state.CurrentStatus, &state.CursorAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func accumulateUptimeDurationTx(ctx context.Context, tx pgx.Tx, projectID int64, status string, start, end time.Time) error {
	start = start.UTC()
	end = end.UTC()
	if !end.After(start) {
		return nil
	}

	for cursor := start; cursor.Before(end); {
		minuteStart := cursor.Truncate(time.Minute)
		minuteEnd := minuteStart.Add(time.Minute)
		if minuteEnd.After(end) {
			minuteEnd = end
		}

		seconds := int(minuteEnd.Sub(cursor).Seconds())
		if seconds <= 0 {
			break
		}

		up := 0
		down := 0
		if status == "up" {
			up = seconds
		} else {
			down = seconds
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO project_uptime_minutes(project_id, bucket_start, up_seconds, down_seconds)
			VALUES($1, $2, $3, $4)
			ON CONFLICT(project_id, bucket_start)
			DO UPDATE SET
				up_seconds = LEAST(60, project_uptime_minutes.up_seconds + EXCLUDED.up_seconds),
				down_seconds = LEAST(60, project_uptime_minutes.down_seconds + EXCLUDED.down_seconds)
		`, projectID, minuteStart, up, down)
		if err != nil {
			return err
		}
		cursor = minuteEnd
	}
	return nil
}

func addTailToSlots(bySlot map[time.Time]uptimeAgg, origin time.Time, bucketSize time.Duration, status string, start, end time.Time) {
	for cursor := start; cursor.Before(end); {
		slotStart := alignToSlot(origin, bucketSize, cursor)
		slotEnd := slotStart.Add(bucketSize)
		if slotEnd.After(end) {
			slotEnd = end
		}
		seconds := int(slotEnd.Sub(cursor).Seconds())
		if seconds <= 0 {
			break
		}

		entry := bySlot[slotStart]
		if status == "up" {
			entry.up += seconds
		} else {
			entry.down += seconds
		}
		bySlot[slotStart] = entry
		cursor = slotEnd
	}
}

func alignToSlot(origin time.Time, bucketSize time.Duration, ts time.Time) time.Time {
	if !ts.After(origin) {
		return origin
	}
	delta := ts.Sub(origin)
	steps := int64(delta / bucketSize)
	return origin.Add(time.Duration(steps) * bucketSize)
}

type uptimeRowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func projectUptimeFreshnessWindow(ctx context.Context, q uptimeRowQuerier, projectID int64) (time.Duration, error) {
	var intervalSec int
	err := q.QueryRow(ctx, `SELECT check_interval_sec FROM projects WHERE id=$1`, projectID).Scan(&intervalSec)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("project %d not found", projectID)
		}
		return 0, err
	}
	if intervalSec <= 0 {
		intervalSec = 30
	}
	freshnessSec := intervalSec * 4
	if freshnessSec < 90 {
		freshnessSec = 90
	}
	if freshnessSec > 3600 {
		freshnessSec = 3600
	}
	return time.Duration(freshnessSec) * time.Second, nil
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

type UpdateFixParams struct {
	Name                  string `json:"name"`
	Type                  string `json:"type"`
	ScriptPath            string `json:"script_path"`
	SupportedErrorPattern string `json:"supported_error_pattern"`
	TimeoutSec            int    `json:"timeout_sec"`
}

func (s *Store) UpdateFix(ctx context.Context, fixID int64, p UpdateFixParams) (Fix, error) {
	if strings.TrimSpace(p.Name) == "" {
		return Fix{}, errors.New("fix name is required")
	}
	if strings.TrimSpace(p.ScriptPath) == "" {
		return Fix{}, errors.New("script_path is required")
	}
	if strings.TrimSpace(p.SupportedErrorPattern) == "" {
		return Fix{}, errors.New("supported_error_pattern is required")
	}
	if p.Type == "" {
		p.Type = "any"
	}
	if p.TimeoutSec <= 0 {
		p.TimeoutSec = 30
	}
	var f Fix
	err := s.pool.QueryRow(ctx, `
		UPDATE fixes
		SET name=$2, type=$3, script_path=$4, supported_error_pattern=$5, timeout_sec=$6
		WHERE id=$1
		RETURNING id, name, type, script_path, supported_error_pattern, timeout_sec
	`, fixID,
		strings.TrimSpace(p.Name),
		strings.TrimSpace(p.Type),
		strings.TrimSpace(p.ScriptPath),
		strings.TrimSpace(p.SupportedErrorPattern),
		p.TimeoutSec,
	).Scan(&f.ID, &f.Name, &f.Type, &f.ScriptPath, &f.SupportedErrorPattern, &f.TimeoutSec)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Fix{}, fmt.Errorf("fix %d not found", fixID)
		}
		return Fix{}, err
	}
	return f, nil
}

func (s *Store) DeleteFix(ctx context.Context, fixID int64) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM fixes WHERE id=$1`, fixID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("fix %d not found", fixID)
	}
	return nil
}

func (s *Store) DetachFixFromProject(ctx context.Context, projectID, fixID int64) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM project_fixes WHERE project_id=$1 AND fix_id=$2`, projectID, fixID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("fix %d not attached to project %d", fixID, projectID)
	}
	return nil
}

type SMTPProfile struct {
	ID                int64
	Host              string
	Port              int
	Username          string
	PasswordEncrypted string
	FromEmail         string
}

type SMTPProfileSummary struct {
	ID        int64  `json:"id"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	FromEmail string `json:"from_email"`
}

func (s *Store) ListSMTPProfiles(ctx context.Context) ([]SMTPProfileSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, host, port, username, from_email
		FROM smtp_profiles
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]SMTPProfileSummary, 0)
	for rows.Next() {
		var item SMTPProfileSummary
		if err := rows.Scan(&item.ID, &item.Host, &item.Port, &item.Username, &item.FromEmail); err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, rows.Err()
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
