package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kraken/internal/autofix"
	"kraken/internal/config"
	"kraken/internal/db"
	"kraken/internal/incident"
	"kraken/internal/monitor"
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

	autofixEngine := autofix.NewEngine(cfg.FixScriptsDir, cfg.AllowedFixCommands)
	incSvc := incident.NewService(store, q, autofixEngine, time.Duration(cfg.AlertCooldownSec)*time.Second)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Println("worker started")
	for {
		select {
		case <-stop:
			log.Println("worker stopping")
			return
		default:
		}

		job, err := q.DequeueCheck(ctx, 5*time.Second)
		if err != nil {
			if err == queue.ErrNoJob {
				continue
			}
			log.Printf("dequeue failed: %v", err)
			continue
		}

		checkCtx, err := store.GetCheckContext(ctx, job.CheckID)
		if err != nil {
			log.Printf("load check context failed: %v", err)
			continue
		}

		result := monitor.RunCheck(ctx, checkCtx.Type, checkCtx.Target, checkCtx.TimeoutMs, checkCtx.ExpectedStatus)
		if err := incSvc.HandleCheckResult(ctx, checkCtx, result); err != nil {
			log.Printf("handle check result failed for check %d: %v", checkCtx.ID, err)
			continue
		}

		if result.Healthy {
			log.Printf("check %d healthy (%dms)", checkCtx.ID, result.ResponseTimeMs)
		} else {
			log.Printf("check %d failed: %s", checkCtx.ID, result.ErrorMessage)
		}
	}
}
