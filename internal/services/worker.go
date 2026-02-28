package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"kraken/internal/autofix"
	"kraken/internal/db"
	"kraken/internal/incident"
	"kraken/internal/monitor"
	"kraken/internal/queue"
)

type Worker struct {
	Store         *db.Store
	Queue         *queue.RedisQueue
	AutofixEngine *autofix.Engine
	Incident      *incident.Service
	Log           *log.Logger
}

func (w *Worker) Run(ctx context.Context) {
	if w.Log == nil {
		w.Log = log.Default()
	}
	w.Log.Println("worker started")

	for {
		select {
		case <-ctx.Done():
			w.Log.Println("worker stopping")
			return
		default:
		}

		fixJob, err := w.Queue.DequeueFix(ctx, 1*time.Second)
		if err == nil {
			w.handleFixJob(ctx, fixJob)
			continue
		}
		if err != nil && err != queue.ErrNoJob {
			w.Log.Printf("dequeue fix failed: %v", err)
		}

		job, err := w.Queue.DequeueCheck(ctx, 4*time.Second)
		if err != nil {
			if err == queue.ErrNoJob {
				continue
			}
			w.Log.Printf("dequeue check failed: %v", err)
			continue
		}
		w.handleCheckJob(ctx, job)
	}
}

func (w *Worker) handleCheckJob(ctx context.Context, job queue.CheckJob) {
	checkCtx, err := w.Store.GetCheckContext(ctx, job.CheckID)
	if err != nil {
		w.Log.Printf("load check context failed: %v", err)
		return
	}

	result := monitor.RunCheck(ctx, checkCtx.Type, checkCtx.Target, checkCtx.TimeoutMs, checkCtx.ExpectedStatus)
	if err := w.Incident.HandleCheckResult(ctx, checkCtx, result); err != nil {
		w.Log.Printf("handle check result failed for check %d: %v", checkCtx.ID, err)
		return
	}

	if result.Healthy {
		w.Log.Printf("check %d healthy (%dms)", checkCtx.ID, result.ResponseTimeMs)
	} else {
		w.Log.Printf("check %d failed: %s", checkCtx.ID, result.ErrorMessage)
	}
}

func (w *Worker) handleFixJob(ctx context.Context, job queue.FixJob) {
	fix, err := w.Store.GetProjectFix(ctx, job.ProjectID, job.FixID)
	if err != nil {
		w.Log.Printf("load fix failed: %v", err)
		_ = w.Store.InsertLog(ctx, job.ProjectID, "error", "manual fix lookup failed: "+err.Error())
		return
	}
	if fix == nil {
		w.Log.Printf("manual fix not found project=%d fix=%d", job.ProjectID, job.FixID)
		_ = w.Store.InsertLog(ctx, job.ProjectID, "error", fmt.Sprintf("manual fix %d not found", job.FixID))
		return
	}

	_ = w.Store.InsertLog(ctx, job.ProjectID, "warn", fmt.Sprintf("manual fix triggered: %s by %s", fix.Name, job.RequestedBy))
	result, execErr := w.AutofixEngine.Execute(ctx, autofix.FixDefinition{
		Name:       fix.Name,
		ScriptPath: fix.ScriptPath,
		TimeoutSec: fix.TimeoutSec,
	})
	if execErr != nil {
		w.Log.Printf("manual fix failed project=%d fix=%d: %v", job.ProjectID, job.FixID, execErr)
		_ = w.Store.InsertLog(ctx, job.ProjectID, "error", fmt.Sprintf("manual fix %s failed: %s", fix.Name, result.Output))
		return
	}
	w.Log.Printf("manual fix succeeded project=%d fix=%d", job.ProjectID, job.FixID)
	_ = w.Store.InsertLog(ctx, job.ProjectID, "warn", fmt.Sprintf("manual fix %s succeeded: %s", fix.Name, result.Output))
}

func (w *Worker) Validate() error {
	if w.Store == nil {
		return fmt.Errorf("worker store is nil")
	}
	if w.Queue == nil {
		return fmt.Errorf("worker queue is nil")
	}
	if w.AutofixEngine == nil {
		return fmt.Errorf("worker autofix engine is nil")
	}
	if w.Incident == nil {
		return fmt.Errorf("worker incident service is nil")
	}
	return nil
}
