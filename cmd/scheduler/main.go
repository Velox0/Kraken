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

	ticker := time.NewTicker(time.Duration(cfg.SchedulerTickSec) * time.Second)
	defer ticker.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("scheduler started (tick=%ds)", cfg.SchedulerTickSec)
	for {
		select {
		case <-stop:
			log.Println("scheduler stopping")
			return
		case <-ticker.C:
			if err := enqueueDueChecks(ctx, store, q); err != nil {
				log.Printf("scheduler cycle failed: %v", err)
			}
		}
	}
}

func enqueueDueChecks(ctx context.Context, store *db.Store, q *queue.RedisQueue) error {
	dueProjects, err := store.AcquireDueProjects(ctx, 200)
	if err != nil {
		return err
	}
	if len(dueProjects) == 0 {
		return nil
	}

	projectIDs := make([]int64, 0, len(dueProjects))
	for _, p := range dueProjects {
		projectIDs = append(projectIDs, p.ID)
	}
	checks, err := store.ListChecksForProjects(ctx, projectIDs)
	if err != nil {
		return err
	}

	for _, check := range checks {
		if err := q.EnqueueCheck(ctx, queue.CheckJob{CheckID: check.ID, Reason: "scheduled"}); err != nil {
			return err
		}
	}
	if len(checks) > 0 {
		log.Printf("enqueued %d checks for %d due projects", len(checks), len(dueProjects))
	}
	return nil
}
