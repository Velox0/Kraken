package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kraken/internal/config"
	"kraken/internal/db"
	"kraken/internal/notifier"
	"kraken/internal/queue"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	store, err := db.New(ctx, cfg.PostgresURL)
	if err != nil {
		log.Fatalf("db init failed: %v", err)
	}
	defer store.Close()

	q := queue.NewRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	defer q.Close()
	if err := q.Ping(ctx); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}

	smtpClient := notifier.NewSMTPClient()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Println("notifier started")
	for {
		select {
		case <-stop:
			log.Println("notifier stopping")
			return
		default:
		}

		job, err := q.DequeueEmail(ctx, 5*time.Second)
		if err != nil {
			if err == queue.ErrNoJob {
				continue
			}
			log.Printf("dequeue failed: %v", err)
			continue
		}

		profile, err := store.GetSMTPProfile(ctx, job.SMTPProfileID)
		if err != nil {
			log.Printf("load smtp profile failed: %v", err)
			continue
		}
		if profile == nil {
			log.Printf("smtp profile not found: %d", job.SMTPProfileID)
			continue
		}

		err = smtpClient.Send(notifier.SMTPProfile{
			Host:              profile.Host,
			Port:              profile.Port,
			Username:          profile.Username,
			PasswordEncrypted: profile.PasswordEncrypted,
			FromEmail:         profile.FromEmail,
		}, job.To, job.Subject, job.Body)
		if err != nil {
			log.Printf("send email failed: %v", err)
			continue
		}
		log.Printf("alert sent to %d recipient(s)", len(job.To))
	}
}
