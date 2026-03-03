package autofix

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type FixDefinition struct {
	Name       string
	ScriptPath string
	TimeoutSec int
}

type Engine struct {
	scriptsDir       string
	allowedCmdLookup map[string]struct{}
}

func NewEngine(scriptsDir string, allowedCommands []string) *Engine {
	lookup := make(map[string]struct{}, len(allowedCommands))
	for _, cmd := range allowedCommands {
		normalized := normalizeAllowedCommand(cmd)
		if normalized == "" {
			continue
		}
		lookup[normalized] = struct{}{}
	}
	return &Engine{
		scriptsDir:       scriptsDir,
		allowedCmdLookup: lookup,
	}
}

type Result struct {
	Success bool
	Output  string
}

func (e *Engine) Execute(ctx context.Context, fix FixDefinition) (Result, error) {
	timeoutSec := fix.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	fixCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	resolvedPath, err := e.resolvePath(fix.ScriptPath)
	if err != nil {
		return Result{}, err
	}

	runner, err := resolveRunner(runtime.GOOS, resolvedPath)
	if err != nil {
		return Result{}, err
	}
	if _, ok := e.allowedCmdLookup[runner.allowlistKey]; !ok {
		return Result{}, fmt.Errorf("%s command is not in allowlist", runner.allowlistKey)
	}

	args := append([]string{}, runner.args...)
	args = append(args, resolvedPath)
	cmd := exec.CommandContext(fixCtx, runner.command, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	if err != nil {
		if output == "" {
			output = err.Error()
		}
		return Result{Success: false, Output: truncate(output, 3000)}, err
	}
	return Result{Success: true, Output: truncate(output, 3000)}, nil
}

func (e *Engine) resolvePath(scriptPath string) (string, error) {
	if strings.TrimSpace(scriptPath) == "" {
		return "", fmt.Errorf("script path is empty")
	}
	path := scriptPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.scriptsDir, scriptPath)
	}
	cleaned := filepath.Clean(path)
	base := filepath.Clean(e.scriptsDir)

	// Keep scripts constrained to the configured scripts directory.
	rel, err := filepath.Rel(base, cleaned)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("script path escapes scripts directory")
	}
	return cleaned, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

type runnerSpec struct {
	allowlistKey string
	command      string
	args         []string
}

func resolveRunner(goos, scriptPath string) (runnerSpec, error) {
	ext := strings.ToLower(filepath.Ext(scriptPath))
	switch goos {
	case "windows":
		switch ext {
		case ".bat", ".cmd":
			return runnerSpec{
				allowlistKey: "cmd",
				command:      "cmd.exe",
				args:         []string{"/C"},
			}, nil
		case ".sh", "":
			return runnerSpec{
				allowlistKey: "bash",
				command:      "bash",
			}, nil
		default:
			return runnerSpec{}, fmt.Errorf("unsupported script extension %q on windows (supported: .bat, .cmd, .sh)", ext)
		}
	default:
		switch ext {
		case ".bat", ".cmd":
			return runnerSpec{}, fmt.Errorf("%s scripts can only run on windows workers", ext)
		default:
			return runnerSpec{
				allowlistKey: "bash",
				command:      "bash",
			}, nil
		}
	}
}

func normalizeAllowedCommand(cmd string) string {
	c := strings.ToLower(strings.TrimSpace(cmd))
	c = strings.TrimSuffix(c, ".exe")
	return c
}
