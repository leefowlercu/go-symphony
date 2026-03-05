package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeWorkflow(t *testing.T, dir string, content string) string {
	t.Helper()
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	return path
}

func TestInitAppliesDefaultsAndOverrides(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "env-token")
	workflow := `---
tracker:
  kind: linear
  project_slug: demo
  active_states: "Todo, In Progress"
polling:
  interval_ms: "1234"
workspace:
  root: "$TEST_WORKSPACE_ROOT"
agent:
  max_concurrent_agents: "7"
  max_concurrent_agents_by_state:
    todo: "2"
    bad: "0"
codex:
  command: codex app-server
---

Work {{ issue.identifier }}`
	dir := t.TempDir()
	t.Setenv("TEST_WORKSPACE_ROOT", filepath.Join(dir, "ws"))
	path := writeWorkflow(t, dir, workflow)

	if err := Init(InitOptions{WorkflowPath: path}); err != nil {
		t.Fatalf("init config: %v", err)
	}
	cfg := Get()
	if cfg.Tracker.APIKey != "env-token" {
		t.Fatalf("expected env api key, got %q", cfg.Tracker.APIKey)
	}
	if cfg.Polling.IntervalMS != 1234 {
		t.Fatalf("expected polling.interval_ms=1234, got %d", cfg.Polling.IntervalMS)
	}
	if len(cfg.Tracker.ActiveStates) != 2 || cfg.Tracker.ActiveStates[0] != "Todo" {
		t.Fatalf("unexpected active states: %#v", cfg.Tracker.ActiveStates)
	}
	if cfg.Agent.MaxConcurrentAgentsByState["todo"] != 2 {
		t.Fatalf("expected todo state limit 2, got %#v", cfg.Agent.MaxConcurrentAgentsByState)
	}
	if _, exists := cfg.Agent.MaxConcurrentAgentsByState["bad"]; exists {
		t.Fatalf("expected invalid state entry to be ignored: %#v", cfg.Agent.MaxConcurrentAgentsByState)
	}
}

func TestReloadKeepsLastKnownGoodOnInvalidWorkflow(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "token")
	dir := t.TempDir()
	path := writeWorkflow(t, dir, `---
tracker:
  kind: linear
  project_slug: first
codex:
  command: codex app-server
---
Prompt {{ issue.identifier }}`)
	if err := Init(InitOptions{WorkflowPath: path}); err != nil {
		t.Fatalf("init config: %v", err)
	}
	before := Get()

	if err := os.WriteFile(path, []byte("---\ntracker: [bad\n---\nprompt"), 0o644); err != nil {
		t.Fatalf("write invalid workflow: %v", err)
	}
	if err := Reload(); err == nil {
		t.Fatal("expected reload error")
	}
	after := Get()
	if after.Tracker.ProjectSlug != before.Tracker.ProjectSlug {
		t.Fatalf("expected previous config to stay active, before=%q after=%q", before.Tracker.ProjectSlug, after.Tracker.ProjectSlug)
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}
	if got := ExpandPath("~/demo"); !strings.HasPrefix(got, home) {
		t.Fatalf("expected home-expanded path, got %q", got)
	}
	t.Setenv("TEST_PATH_VAR", "/tmp/demo")
	if got := ExpandPath("$TEST_PATH_VAR"); got != "/tmp/demo" {
		t.Fatalf("expected env-expanded path, got %q", got)
	}
}
