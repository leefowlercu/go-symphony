package workspace

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/leefowlercu/go-symphony/internal/config"
)

func runHook(ctx context.Context, logger *slog.Logger, name string, script *string, cwd string, timeoutMS int, fatal bool) error {
	if script == nil || strings.TrimSpace(*script) == "" {
		return nil
	}
	hookCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(hookCtx, "sh", "-lc", *script)
	cmd.Dir = cwd
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	logger.Info("hook started", "hook", name, "cwd", cwd)
	err := cmd.Run()
	output := truncateHookOutput(out.String(), 2000)
	if err != nil {
		logger.Error("hook failed", "hook", name, "error", err, "output", output)
		if fatal {
			return fmt.Errorf("hook %s failed; %w", name, err)
		}
		return nil
	}
	logger.Info("hook completed", "hook", name)
	return nil
}

func truncateHookOutput(in string, max int) string {
	trim := strings.TrimSpace(in)
	if len(trim) <= max {
		return trim
	}
	return trim[:max] + "...<truncated>"
}

func runAfterCreate(ctx context.Context, logger *slog.Logger, hooks config.HooksConfig, path string) error {
	return runHook(ctx, logger, "after_create", hooks.AfterCreate, path, hooks.TimeoutMS, true)
}

func runBeforeRun(ctx context.Context, logger *slog.Logger, hooks config.HooksConfig, path string) error {
	return runHook(ctx, logger, "before_run", hooks.BeforeRun, path, hooks.TimeoutMS, true)
}

func runAfterRun(ctx context.Context, logger *slog.Logger, hooks config.HooksConfig, path string) {
	_ = runHook(ctx, logger, "after_run", hooks.AfterRun, path, hooks.TimeoutMS, false)
}

func runBeforeRemove(ctx context.Context, logger *slog.Logger, hooks config.HooksConfig, path string) {
	_ = runHook(ctx, logger, "before_remove", hooks.BeforeRemove, path, hooks.TimeoutMS, false)
}
