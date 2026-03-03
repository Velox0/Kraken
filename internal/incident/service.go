package incident

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"kraken/internal/autofix"
	"kraken/internal/db"
	"kraken/internal/monitor"
	"kraken/internal/queue"
)

// EmailConfig holds env-based SMTP credentials for escalation emails.
type EmailConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

type Service struct {
	store         *db.Store
	queue         *queue.RedisQueue
	autofixEngine *autofix.Engine
	alertCooldown time.Duration
	emailCfg      EmailConfig
}

func NewService(store *db.Store, q *queue.RedisQueue, fx *autofix.Engine, alertCooldown time.Duration, emailCfg EmailConfig) *Service {
	return &Service{
		store:         store,
		queue:         q,
		autofixEngine: fx,
		alertCooldown: alertCooldown,
		emailCfg:      emailCfg,
	}
}

func (s *Service) HandleCheckResult(ctx context.Context, check db.CheckContext, result monitor.Result) error {
	status := "healthy"
	if !result.Healthy {
		status = "failed"
	}

	if err := s.store.InsertCheckRun(ctx, check.ID, check.ProjectID, status, result.ResponseTimeMs, result.ErrorMessage); err != nil {
		return err
	}

	if result.Healthy {
		return s.handleHealthy(ctx, check, result)
	}
	return s.handleFailure(ctx, check, result)
}

func (s *Service) handleHealthy(ctx context.Context, check db.CheckContext, result monitor.Result) error {
	if err := s.store.SetProjectHealth(ctx, check.ProjectID, 0, "healthy"); err != nil {
		return err
	}
	_ = s.store.InsertLog(ctx, check.ProjectID, "info", fmt.Sprintf("check %d healthy (%dms)", check.ID, result.ResponseTimeMs))

	openIncident, err := s.store.GetOpenIncident(ctx, check.ProjectID)
	if err != nil {
		return err
	}
	if openIncident == nil {
		return nil
	}
	if err := s.store.ResolveIncident(ctx, openIncident.ID); err != nil {
		return err
	}
	_ = s.store.InsertLog(ctx, check.ProjectID, "info", fmt.Sprintf("incident %d resolved", openIncident.ID))
	return s.enqueueAlert(ctx, check, openIncident.ID, "resolved", "none")
}

func (s *Service) handleFailure(ctx context.Context, check db.CheckContext, result monitor.Result) error {
	health, err := s.store.GetProjectHealth(ctx, check.ProjectID)
	if err != nil {
		return err
	}
	consecutive := health.ConsecutiveFailures + 1
	if err := s.store.SetProjectHealth(ctx, check.ProjectID, consecutive, "failed"); err != nil {
		return err
	}
	_ = s.store.InsertLog(ctx, check.ProjectID, "error", fmt.Sprintf("check %d failed (%d/%d): %s", check.ID, consecutive, check.FailureThreshold, result.ErrorMessage))

	if consecutive < check.FailureThreshold {
		return nil
	}

	existing, err := s.store.GetOpenIncident(ctx, check.ProjectID)
	if err != nil {
		return err
	}

	newlyOpened := existing == nil
	incidentID := int64(0)
	autofixAttempts := 0
	if newlyOpened {
		inc, err := s.store.CreateIncident(ctx, check.ProjectID, result.ErrorMessage)
		if err != nil {
			return err
		}
		incidentID = inc.ID
		autofixAttempts = inc.AutofixAttempts
		_ = s.store.InsertLog(ctx, check.ProjectID, "warn", fmt.Sprintf("incident %d opened", inc.ID))
	} else {
		incidentID = existing.ID
		autofixAttempts = existing.AutofixAttempts
	}

	autofixStatus := "not_attempted"
	maxRetries := check.MaxAutofixRetries
	if check.AutofixEnabled && maxRetries > 0 {
		if autofixAttempts < maxRetries {
			autofixStatus = s.runAutofix(ctx, check, result.ErrorMessage)
			newCount, err := s.store.IncrementIncidentAutofixAttempts(ctx, incidentID)
			if err != nil {
				_ = s.store.InsertLog(ctx, check.ProjectID, "error", "failed to increment autofix attempts: "+err.Error())
			}
			if newCount >= maxRetries {
				_ = s.store.InsertLog(ctx, check.ProjectID, "warn", fmt.Sprintf("autofix retry limit (%d) reached for incident %d", maxRetries, incidentID))
				s.sendAutofixExceededEmail(ctx, check, incidentID, newCount)
			}
		} else {
			autofixStatus = "limit_exceeded"
		}
	} else if check.AutofixEnabled && maxRetries == 0 {
		// maxRetries == 0 means unlimited retries
		autofixStatus = s.runAutofix(ctx, check, result.ErrorMessage)
		_, _ = s.store.IncrementIncidentAutofixAttempts(ctx, incidentID)
	}

	eventType := "repeated"
	if newlyOpened {
		eventType = "opened"
	}
	if !s.shouldSendAlert(existing, newlyOpened) {
		return nil
	}
	if err := s.enqueueAlert(ctx, check, incidentID, eventType, autofixStatus); err != nil {
		return err
	}
	return s.store.UpdateIncidentAlertTime(ctx, incidentID)
}

func (s *Service) shouldSendAlert(existing *db.Incident, newlyOpened bool) bool {
	if newlyOpened {
		return true
	}
	if existing == nil {
		return true
	}
	if existing.LastAlertSentAt == nil {
		return true
	}
	return time.Since(*existing.LastAlertSentAt) >= s.alertCooldown
}

func (s *Service) runAutofix(ctx context.Context, check db.CheckContext, errMessage string) string {
	fix, err := s.store.FindMatchingFix(ctx, check.ProjectID, check.Type, errMessage)
	if err != nil {
		_ = s.store.InsertLog(ctx, check.ProjectID, "error", "autofix lookup failed: "+err.Error())
		return "lookup_failed"
	}
	if fix == nil {
		_ = s.store.InsertLog(ctx, check.ProjectID, "info", "autofix enabled but no matching fix found")
		return "not_found"
	}

	result, execErr := s.autofixEngine.Execute(ctx, autofix.FixDefinition{
		Name:       fix.Name,
		ScriptPath: fix.ScriptPath,
		TimeoutSec: fix.TimeoutSec,
	})
	if execErr != nil {
		_ = s.store.InsertLog(ctx, check.ProjectID, "error", fmt.Sprintf("autofix %q failed: %s", fix.Name, result.Output))
		return "failed"
	}
	_ = s.store.InsertLog(ctx, check.ProjectID, "warn", fmt.Sprintf("autofix %q succeeded: %s", fix.Name, result.Output))
	return "success"
}

func (s *Service) enqueueAlert(ctx context.Context, check db.CheckContext, incidentID int64, eventType, autofixStatus string) error {
	if len(check.AlertEmails) == 0 {
		_ = s.store.InsertLog(ctx, check.ProjectID, "warn", "alert skipped (no recipients configured)")
		return nil
	}

	smtpProfileID := int64(0) // Use env/default smtp when project smtp profile is not selected.
	if check.ProjectSMTPID != nil && *check.ProjectSMTPID > 0 {
		smtpProfileID = *check.ProjectSMTPID
	}

	subject := buildSubject(eventType, check.ProjectDomain)
	body := strings.Join([]string{
		fmt.Sprintf("Project: %s", check.ProjectName),
		fmt.Sprintf("Domain: %s", check.ProjectDomain),
		fmt.Sprintf("Event: %s", eventType),
		fmt.Sprintf("Incident ID: %d", incidentID),
		fmt.Sprintf("Timestamp: %s", time.Now().UTC().Format(time.RFC3339)),
		fmt.Sprintf("Autofix: %s", autofixStatus),
	}, "\n")

	return s.queue.EnqueueEmail(ctx, queue.EmailJob{
		SMTPProfileID: smtpProfileID,
		To:            check.AlertEmails,
		Subject:       subject,
		Body:          body,
	})
}

func buildSubject(eventType, domain string) string {
	switch eventType {
	case "opened":
		return fmt.Sprintf("[DOWN] %s is unreachable", domain)
	case "resolved":
		return fmt.Sprintf("[RESOLVED] %s recovered", domain)
	default:
		return fmt.Sprintf("[DOWN][REPEATED] %s still failing", domain)
	}
}

// sendAutofixExceededEmail sends an escalation email using env-based SMTP
// credentials when autofix retries are exhausted. Falls back to the project's
// SMTP profile queue path if env creds are not configured.
func (s *Service) sendAutofixExceededEmail(ctx context.Context, check db.CheckContext, incidentID int64, attempts int) {
	recipients := check.AlertEmails
	if len(recipients) == 0 {
		_ = s.store.InsertLog(ctx, check.ProjectID, "warn", "autofix-exceeded email skipped (no recipients)")
		return
	}

	subject := fmt.Sprintf("[AUTOFIX LIMIT] %s – autofix retries exhausted", check.ProjectDomain)
	body := strings.Join([]string{
		fmt.Sprintf("Project: %s", check.ProjectName),
		fmt.Sprintf("Domain: %s", check.ProjectDomain),
		fmt.Sprintf("Incident ID: %d", incidentID),
		fmt.Sprintf("Autofix attempts: %d", attempts),
		fmt.Sprintf("Max retries: %d", check.MaxAutofixRetries),
		fmt.Sprintf("Timestamp: %s", time.Now().UTC().Format(time.RFC3339)),
		"",
		"Automatic fixes have been exhausted. Manual intervention required.",
	}, "\n")

	// Prefer env-based SMTP if configured
	if s.emailCfg.User != "" && s.emailCfg.Pass != "" {
		addr := fmt.Sprintf("%s:%d", s.emailCfg.Host, s.emailCfg.Port)
		auth := smtp.PlainAuth("", s.emailCfg.User, s.emailCfg.Pass, s.emailCfg.Host)
		from := s.emailCfg.From
		if strings.TrimSpace(from) == "" {
			from = s.emailCfg.User
		}

		msg := strings.Builder{}
		msg.WriteString("From: " + from + "\r\n")
		msg.WriteString("To: " + strings.Join(recipients, ",") + "\r\n")
		msg.WriteString("Subject: " + subject + "\r\n")
		msg.WriteString("MIME-Version: 1.0\r\n")
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		msg.WriteString("\r\n")
		msg.WriteString(body)

		if err := smtp.SendMail(addr, auth, from, recipients, []byte(msg.String())); err != nil {
			_ = s.store.InsertLog(ctx, check.ProjectID, "error", "autofix-exceeded email (env smtp) failed: "+err.Error())
		} else {
			_ = s.store.InsertLog(ctx, check.ProjectID, "warn", "autofix-exceeded escalation email sent via env smtp")
		}
		return
	}

	// Fallback: enqueue through project SMTP profile
	if check.ProjectSMTPID != nil {
		_ = s.queue.EnqueueEmail(ctx, queue.EmailJob{
			SMTPProfileID: *check.ProjectSMTPID,
			To:            recipients,
			Subject:       subject,
			Body:          body,
		})
		_ = s.store.InsertLog(ctx, check.ProjectID, "warn", "autofix-exceeded escalation email enqueued via project smtp")
	} else {
		_ = s.store.InsertLog(ctx, check.ProjectID, "warn", "autofix-exceeded email skipped (no smtp configured)")
	}
}
