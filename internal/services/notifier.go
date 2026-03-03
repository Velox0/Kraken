package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"kraken/internal/db"
	"kraken/internal/notifier"
	"kraken/internal/queue"
)

type Notifier struct {
	Store       *db.Store
	Queue       *queue.RedisQueue
	SMTPClient  *notifier.SMTPClient
	DefaultSMTP notifier.SMTPProfile
	Log         *log.Logger
}

func (n *Notifier) Run(ctx context.Context) {
	if n.Log == nil {
		n.Log = log.Default()
	}
	n.Log.Println("notifier started")

	for {
		select {
		case <-ctx.Done():
			n.Log.Println("notifier stopping")
			return
		default:
		}

		job, err := n.Queue.DequeueEmail(ctx, 5*time.Second)
		if err != nil {
			if err == queue.ErrNoJob {
				continue
			}
			n.Log.Printf("dequeue failed: %v", err)
			continue
		}

		smtpProfile, err := n.resolveSMTPProfile(ctx, job.SMTPProfileID)
		if err != nil {
			n.Log.Printf("resolve smtp profile failed: %v", err)
			continue
		}

		err = n.SMTPClient.Send(smtpProfile, job.To, job.Subject, job.Body)
		if err != nil {
			n.Log.Printf("send email failed: %v", err)
			continue
		}
		n.Log.Printf("alert sent to %d recipient(s)", len(job.To))
	}
}

func (n *Notifier) resolveSMTPProfile(ctx context.Context, profileID int64) (notifier.SMTPProfile, error) {
	if profileID <= 0 {
		if n.hasDefaultSMTP() {
			return n.DefaultSMTP, nil
		}
		return notifier.SMTPProfile{}, fmt.Errorf("no default env smtp configured")
	}

	profile, err := n.Store.GetSMTPProfile(ctx, profileID)
	if err != nil {
		return notifier.SMTPProfile{}, err
	}
	if profile == nil {
		if n.hasDefaultSMTP() {
			n.Log.Printf("smtp profile %d missing, falling back to env default smtp", profileID)
			return n.DefaultSMTP, nil
		}
		return notifier.SMTPProfile{}, fmt.Errorf("smtp profile not found: %d", profileID)
	}

	return notifier.SMTPProfile{
		Host:              profile.Host,
		Port:              profile.Port,
		Username:          profile.Username,
		PasswordEncrypted: profile.PasswordEncrypted,
		FromEmail:         profile.FromEmail,
	}, nil
}

func (n *Notifier) hasDefaultSMTP() bool {
	return n.DefaultSMTP.Host != "" &&
		n.DefaultSMTP.Port > 0 &&
		n.DefaultSMTP.Username != "" &&
		n.DefaultSMTP.PasswordEncrypted != "" &&
		n.DefaultSMTP.FromEmail != ""
}

func (n *Notifier) Validate() error {
	if n.Store == nil {
		return fmt.Errorf("notifier store is nil")
	}
	if n.Queue == nil {
		return fmt.Errorf("notifier queue is nil")
	}
	if n.SMTPClient == nil {
		return fmt.Errorf("notifier smtp client is nil")
	}
	return nil
}
