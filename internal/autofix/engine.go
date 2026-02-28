package autofix

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
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
		lookup[strings.TrimSpace(cmd)] = struct{}{}
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
	if _, ok := e.allowedCmdLookup["bash"]; !ok {
		return Result{}, fmt.Errorf("bash command is not in allowlist")
	}

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

	cmd := exec.CommandContext(fixCtx, "bash", resolvedPath)
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
