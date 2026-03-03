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
	"kraken/internal/queue"
	"kraken/internal/services"
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
	incSvc := incident.NewService(store, q, autofixEngine, time.Duration(cfg.AlertCooldownSec)*time.Second, incident.EmailConfig{
		Host: cfg.EmailHost,
		Port: cfg.EmailPort,
		User: cfg.EmailUser,
		Pass: cfg.EmailPass,
		From: cfg.EmailFrom,
	})

	runner := &services.Worker{
		Store:         store,
		Queue:         q,
		AutofixEngine: autofixEngine,
		Incident:      incSvc,
		Log:           log.Default(),
	}
	if err := runner.Validate(); err != nil {
		log.Fatalf("worker config invalid: %v", err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		cancel()
	}()

	runner.Run(runCtx)
}
