package queue

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	checkQueueKey = "kraken:queue:checks"
	emailQueueKey = "kraken:queue:emails"
)

var ErrNoJob = errors.New("no job available")

type CheckJob struct {
	CheckID    int64     `json:"check_id"`
	EnqueuedAt time.Time `json:"enqueued_at"`
	Reason     string    `json:"reason"`
}

type EmailJob struct {
	SMTPProfileID int64     `json:"smtp_profile_id"`
	To            []string  `json:"to"`
	Subject       string    `json:"subject"`
	Body          string    `json:"body"`
	EnqueuedAt    time.Time `json:"enqueued_at"`
}

type RedisQueue struct {
	client *redis.Client
}

func NewRedis(addr, password string, db int) *RedisQueue {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &RedisQueue{client: client}
}

func (q *RedisQueue) Close() error {
	return q.client.Close()
}

func (q *RedisQueue) Ping(ctx context.Context) error {
	return q.client.Ping(ctx).Err()
}

func (q *RedisQueue) EnqueueCheck(ctx context.Context, job CheckJob) error {
	if job.EnqueuedAt.IsZero() {
		job.EnqueuedAt = time.Now().UTC()
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.client.LPush(ctx, checkQueueKey, payload).Err()
}

func (q *RedisQueue) EnqueueEmail(ctx context.Context, job EmailJob) error {
	if job.EnqueuedAt.IsZero() {
		job.EnqueuedAt = time.Now().UTC()
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.client.LPush(ctx, emailQueueKey, payload).Err()
}

func (q *RedisQueue) DequeueCheck(ctx context.Context, timeout time.Duration) (CheckJob, error) {
	items, err := q.client.BRPop(ctx, timeout, checkQueueKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return CheckJob{}, ErrNoJob
		}
		return CheckJob{}, err
	}
	if len(items) != 2 {
		return CheckJob{}, ErrNoJob
	}
	var job CheckJob
	if err := json.Unmarshal([]byte(items[1]), &job); err != nil {
		return CheckJob{}, err
	}
	return job, nil
}

func (q *RedisQueue) DequeueEmail(ctx context.Context, timeout time.Duration) (EmailJob, error) {
	items, err := q.client.BRPop(ctx, timeout, emailQueueKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return EmailJob{}, ErrNoJob
		}
		return EmailJob{}, err
	}
	if len(items) != 2 {
		return EmailJob{}, ErrNoJob
	}
	var job EmailJob
	if err := json.Unmarshal([]byte(items[1]), &job); err != nil {
		return EmailJob{}, err
	}
	return job, nil
}
