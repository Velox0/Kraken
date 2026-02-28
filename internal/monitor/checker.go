package monitor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Result struct {
	Healthy        bool
	ResponseTimeMs int
	ErrorMessage   string
	StatusCode     int
}

func RunCheck(ctx context.Context, checkType, target string, timeoutMs int, expectedStatus *int) Result {
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond

	switch checkType {
	case "http":
		return runHTTP(ctx, target, timeout, expectedStatus)
	case "tcp":
		return runTCP(ctx, target, timeout)
	case "ping":
		return runPing(ctx, target, timeout)
	default:
		return Result{Healthy: false, ErrorMessage: "unsupported check type: " + checkType}
	}
}

func runHTTP(ctx context.Context, target string, timeout time.Duration, expectedStatus *int) Result {
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "http://" + target
	}
	parsed, err := url.ParseRequestURI(target)
	if err != nil {
		return Result{Healthy: false, ErrorMessage: "invalid URL target"}
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Result{Healthy: false, ErrorMessage: err.Error()}
	}

	started := time.Now()
	resp, err := client.Do(req)
	elapsed := int(time.Since(started).Milliseconds())
	if err != nil {
		return Result{Healthy: false, ResponseTimeMs: elapsed, ErrorMessage: err.Error()}
	}
	defer resp.Body.Close()

	if expectedStatus != nil && resp.StatusCode != *expectedStatus {
		return Result{
			Healthy:        false,
			ResponseTimeMs: elapsed,
			StatusCode:     resp.StatusCode,
			ErrorMessage:   fmt.Sprintf("expected status %d got %d", *expectedStatus, resp.StatusCode),
		}
	}

	if expectedStatus == nil && resp.StatusCode >= 400 {
		return Result{
			Healthy:        false,
			ResponseTimeMs: elapsed,
			StatusCode:     resp.StatusCode,
			ErrorMessage:   fmt.Sprintf("status code %d", resp.StatusCode),
		}
	}

	return Result{Healthy: true, ResponseTimeMs: elapsed, StatusCode: resp.StatusCode}
}

func runTCP(ctx context.Context, target string, timeout time.Duration) Result {
	if _, _, err := net.SplitHostPort(target); err != nil {
		return Result{Healthy: false, ErrorMessage: "tcp target must be host:port"}
	}
	started := time.Now()
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", target)
	elapsed := int(time.Since(started).Milliseconds())
	if err != nil {
		return Result{Healthy: false, ResponseTimeMs: elapsed, ErrorMessage: err.Error()}
	}
	_ = conn.Close()
	return Result{Healthy: true, ResponseTimeMs: elapsed}
}

func runPing(ctx context.Context, target string, timeout time.Duration) Result {
	// Uses system ping so it works without raw socket privileges in the process.
	args := []string{"-c", "1", target}
	if runtime.GOOS == "linux" {
		args = []string{"-c", "1", "-W", fmt.Sprintf("%d", int(timeout.Seconds())), target}
	}

	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	cmd := exec.CommandContext(pingCtx, "ping", args...)
	output, err := cmd.CombinedOutput()
	elapsed := int(time.Since(started).Milliseconds())
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return Result{Healthy: false, ResponseTimeMs: elapsed, ErrorMessage: truncate(msg, 300)}
	}
	return Result{Healthy: true, ResponseTimeMs: elapsed}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
