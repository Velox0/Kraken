package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"kraken/internal/db"
	"kraken/internal/queue"
)

type Scheduler struct {
	Store *db.Store
	Queue *queue.RedisQueue
	Tick  time.Duration
	Log   *log.Logger
}

func (s *Scheduler) Run(ctx context.Context) {
	if s.Log == nil {
		s.Log = log.Default()
	}
	if s.Tick <= 0 {
		s.Tick = 2 * time.Second
	}

	ticker := time.NewTicker(s.Tick)
	defer ticker.Stop()

	s.Log.Printf("scheduler started (tick=%s)", s.Tick)
	for {
		select {
		case <-ctx.Done():
			s.Log.Println("scheduler stopping")
			return
		case <-ticker.C:
			if err := EnqueueDueChecks(ctx, s.Store, s.Queue); err != nil {
				s.Log.Printf("scheduler cycle failed: %v", err)
			}
		}
	}
}

func EnqueueDueChecks(ctx context.Context, store *db.Store, q *queue.RedisQueue) error {
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

func (s *Scheduler) Validate() error {
	if s.Store == nil {
		return fmt.Errorf("scheduler store is nil")
	}
	if s.Queue == nil {
		return fmt.Errorf("scheduler queue is nil")
	}
	return nil
}
