package subcommands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWorkflowPath(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte("prompt"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	resolved, err := ResolveWorkflowPath(nil)
	if err != nil {
		t.Fatalf("resolve default workflow path: %v", err)
	}
	want, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("eval symlinks want: %v", err)
	}
	got, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		t.Fatalf("eval symlinks got: %v", err)
	}
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveWorkflowPathMissing(t *testing.T) {
	if _, err := ResolveWorkflowPath([]string{"/missing/path/WORKFLOW.md"}); err == nil {
		t.Fatal("expected missing workflow path error")
	}
}

func TestResolvePort(t *testing.T) {
	if got := ResolvePort(9000, 7777, true); got != 9000 {
		t.Fatalf("expected cli port precedence, got %d", got)
	}
	if got := ResolvePort(0, 7777, false); got != 7777 {
		t.Fatalf("expected workflow port fallback, got %d", got)
	}
	if got := ResolvePort(0, 0, false); got != 0 {
		t.Fatalf("expected no port, got %d", got)
	}
	if got := ResolvePort(0, 7777, true); got != 0 {
		t.Fatalf("expected explicit --port 0 to disable, got %d", got)
	}
}
