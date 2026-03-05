package workspace

import (
	"context"
	"io/ioutil"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leefowlercu/go-symphony/internal/config"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(ioutil.Discard, nil))
}

func TestEnsureWorkspaceSanitizesIdentifierAndReuses(t *testing.T) {
	dir := t.TempDir()
	hooks := config.HooksConfig{}
	m := NewManager(dir, hooks, testLogger())

	ws1, err := m.EnsureWorkspace(context.Background(), "ABC/123")
	if err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	if !ws1.CreatedNow {
		t.Fatal("expected first ensure to create workspace")
	}
	if strings.Contains(ws1.WorkspaceKey, "/") {
		t.Fatalf("expected sanitized workspace key, got %q", ws1.WorkspaceKey)
	}
	if !strings.HasPrefix(ws1.Path, dir) {
		t.Fatalf("expected workspace under root, path=%q root=%q", ws1.Path, dir)
	}

	ws2, err := m.EnsureWorkspace(context.Background(), "ABC/123")
	if err != nil {
		t.Fatalf("ensure workspace second: %v", err)
	}
	if ws2.CreatedNow {
		t.Fatal("expected second ensure to reuse existing workspace")
	}
}

func TestAfterCreateRunsOnlyOnCreate(t *testing.T) {
	dir := t.TempDir()
	hook := "echo run >> after_create_runs.txt"
	m := NewManager(dir, config.HooksConfig{AfterCreate: &hook, TimeoutMS: 5000}, testLogger())

	if _, err := m.EnsureWorkspace(context.Background(), "ABC-1"); err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	if _, err := m.EnsureWorkspace(context.Background(), "ABC-1"); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "ABC-1", "after_create_runs.txt"))
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected hook to run once, got %d lines (%q)", len(lines), string(data))
	}
}

func TestBeforeRunFailureIsFatal(t *testing.T) {
	dir := t.TempDir()
	beforeRun := "exit 1"
	m := NewManager(dir, config.HooksConfig{BeforeRun: &beforeRun, TimeoutMS: 1000}, testLogger())
	ws, err := m.EnsureWorkspace(context.Background(), "ABC-2")
	if err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	if err := m.RunBeforeRun(context.Background(), ws.Path); err == nil {
		t.Fatal("expected before_run failure")
	}
}

func TestBeforeRemoveFailureIsIgnored(t *testing.T) {
	dir := t.TempDir()
	beforeRemove := "exit 1"
	m := NewManager(dir, config.HooksConfig{BeforeRemove: &beforeRemove, TimeoutMS: 1000}, testLogger())
	if _, err := m.EnsureWorkspace(context.Background(), "ABC-3"); err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	if err := m.RemoveWorkspace(context.Background(), "ABC-3"); err != nil {
		t.Fatalf("remove workspace should ignore before_remove failure: %v", err)
	}
}
