package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/leefowlercu/go-symphony/internal/config"
	"github.com/leefowlercu/go-symphony/internal/domain"
)

type Manager struct {
	root   string
	hooks  config.HooksConfig
	logger *slog.Logger
}

func NewManager(root string, hooks config.HooksConfig, logger *slog.Logger) *Manager {
	return &Manager{root: root, hooks: hooks, logger: logger}
}

func (m *Manager) EnsureWorkspace(ctx context.Context, issueIdentifier string) (domain.Workspace, error) {
	key := sanitizeWorkspaceKey(issueIdentifier)
	path := filepath.Join(m.root, key)
	if err := ensureWithinRoot(m.root, path); err != nil {
		return domain.Workspace{}, err
	}

	createdNow := false
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return domain.Workspace{}, fmt.Errorf("workspace path exists and is not a directory")
		}
	} else {
		if !os.IsNotExist(err) {
			return domain.Workspace{}, fmt.Errorf("stat workspace; %w", err)
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return domain.Workspace{}, fmt.Errorf("create workspace; %w", err)
		}
		createdNow = true
	}

	if createdNow {
		if err := runAfterCreate(ctx, m.logger, m.hooks, path); err != nil {
			return domain.Workspace{}, err
		}
	}

	return domain.Workspace{Path: path, WorkspaceKey: key, CreatedNow: createdNow}, nil
}

func (m *Manager) RemoveWorkspace(ctx context.Context, issueIdentifier string) error {
	key := sanitizeWorkspaceKey(issueIdentifier)
	path := filepath.Join(m.root, key)
	if err := ensureWithinRoot(m.root, path); err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	runBeforeRemove(ctx, m.logger, m.hooks, path)
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove workspace; %w", err)
	}
	return nil
}

func (m *Manager) Hooks() config.HooksConfig {
	return m.hooks
}

func (m *Manager) RunBeforeRun(ctx context.Context, path string) error {
	return runBeforeRun(ctx, m.logger, m.hooks, path)
}

func (m *Manager) RunAfterRun(ctx context.Context, path string) {
	runAfterRun(ctx, m.logger, m.hooks, path)
}
