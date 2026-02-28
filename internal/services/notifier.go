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
	Store      *db.Store
	Queue      *queue.RedisQueue
	SMTPClient *notifier.SMTPClient
	Log        *log.Logger
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

		profile, err := n.Store.GetSMTPProfile(ctx, job.SMTPProfileID)
		if err != nil {
			n.Log.Printf("load smtp profile failed: %v", err)
			continue
		}
		if profile == nil {
			n.Log.Printf("smtp profile not found: %d", job.SMTPProfileID)
			continue
		}

		err = n.SMTPClient.Send(notifier.SMTPProfile{
			Host:              profile.Host,
			Port:              profile.Port,
			Username:          profile.Username,
			PasswordEncrypted: profile.PasswordEncrypted,
			FromEmail:         profile.FromEmail,
		}, job.To, job.Subject, job.Body)
		if err != nil {
			n.Log.Printf("send email failed: %v", err)
			continue
		}
		n.Log.Printf("alert sent to %d recipient(s)", len(job.To))
	}
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
