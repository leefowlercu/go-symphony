package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPromptOnlyWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	content := "You are working on {{ issue.identifier }}."
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	def, err := Load(path)
	if err != nil {
		t.Fatalf("load workflow: %v", err)
	}
	if def.PromptTemplate != content {
		t.Fatalf("unexpected prompt template: %q", def.PromptTemplate)
	}
	if len(def.Config) != 0 {
		t.Fatalf("expected empty config, got: %#v", def.Config)
	}
}

func TestLoadFrontMatterWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	content := `---
tracker:
  kind: linear
  project_slug: demo
---

Work issue {{ issue.identifier }}.`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	def, err := Load(path)
	if err != nil {
		t.Fatalf("load workflow: %v", err)
	}
	tracker, ok := def.Config["tracker"].(map[string]any)
	if !ok {
		t.Fatalf("tracker config missing: %#v", def.Config)
	}
	if got, _ := tracker["kind"].(string); got != "linear" {
		t.Fatalf("expected tracker.kind=linear, got %q", got)
	}
}

func TestLoadMissingWorkflowReturnsTypedError(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "MISSING_WORKFLOW.md"))
	if err == nil {
		t.Fatal("expected error")
	}
	var wfErr *Error
	if !errors.As(err, &wfErr) {
		t.Fatalf("expected workflow error type, got %T", err)
	}
	if wfErr.Code != ErrMissingWorkflowFile {
		t.Fatalf("expected code %s, got %s", ErrMissingWorkflowFile, wfErr.Code)
	}
}

func TestLoadFrontMatterMustBeMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	content := `---
- not-a-map
---
body`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	var wfErr *Error
	if !errors.As(err, &wfErr) {
		t.Fatalf("expected workflow error type, got %T", err)
	}
	if wfErr.Code != ErrWorkflowFrontMatterType {
		t.Fatalf("expected code %s, got %s", ErrWorkflowFrontMatterType, wfErr.Code)
	}
}
