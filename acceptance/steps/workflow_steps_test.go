package steps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/leefowlercu/go-symphony/internal/workflow"
)

type workflowSuite struct {
	dir           string
	path          string
	loaded        workflow.Definition
	loadErr       error
	workflowError *workflow.Error
}

func (s *workflowSuite) aWorkflowFileWithTrackerKind(kind string) error {
	content := fmt.Sprintf(`---
tracker:
  kind: %s
  project_slug: demo
codex:
  command: codex app-server
---

Issue {{ issue.identifier }}`, kind)
	s.path = filepath.Join(s.dir, "WORKFLOW.md")
	return os.WriteFile(s.path, []byte(content), 0o644)
}

func (s *workflowSuite) aMissingWorkflowFilePath() {
	s.path = filepath.Join(s.dir, "MISSING_WORKFLOW.md")
}

func (s *workflowSuite) iLoadTheWorkflowDefinition() {
	s.loaded, s.loadErr = workflow.Load(s.path)
	if s.loadErr != nil {
		_ = errors.As(s.loadErr, &s.workflowError)
	}
}

func (s *workflowSuite) theWorkflowConfigValueEquals(path string, expected string) error {
	if s.loadErr != nil {
		return fmt.Errorf("expected load success, got %v", s.loadErr)
	}
	parts := strings.Split(path, ".")
	if len(parts) != 2 {
		return fmt.Errorf("unsupported path %q", path)
	}
	section, ok := s.loaded.Config[parts[0]].(map[string]any)
	if !ok {
		return fmt.Errorf("missing section %q", parts[0])
	}
	got, _ := section[parts[1]].(string)
	if got != expected {
		return fmt.Errorf("expected %s=%q, got %q", path, expected, got)
	}
	return nil
}

func (s *workflowSuite) theWorkflowPromptContains(fragment string) error {
	if s.loadErr != nil {
		return fmt.Errorf("expected load success, got %v", s.loadErr)
	}
	if !strings.Contains(s.loaded.PromptTemplate, fragment) {
		return fmt.Errorf("prompt missing fragment %q", fragment)
	}
	return nil
}

func (s *workflowSuite) theWorkflowErrorCodeEquals(code string) error {
	if s.workflowError == nil {
		return fmt.Errorf("expected typed workflow error, got %v", s.loadErr)
	}
	if string(s.workflowError.Code) != code {
		return fmt.Errorf("expected code %q, got %q", code, s.workflowError.Code)
	}
	return nil
}

func InitializeScenario(sc *godog.ScenarioContext) {
	s := &workflowSuite{}
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		dir, err := os.MkdirTemp("", "go-symphony-godog-*")
		if err != nil {
			return ctx, err
		}
		s.dir = dir
		s.path = ""
		s.loaded = workflow.Definition{}
		s.loadErr = nil
		s.workflowError = nil
		return ctx, nil
	})
	sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		if s.dir != "" {
			_ = os.RemoveAll(s.dir)
		}
		return ctx, nil
	})
	sc.Given(`^a workflow file with tracker kind "([^"]*)"$`, s.aWorkflowFileWithTrackerKind)
	sc.Given(`^a missing workflow file path$`, s.aMissingWorkflowFilePath)
	sc.When(`^I load the workflow definition$`, s.iLoadTheWorkflowDefinition)
	sc.Then(`^the workflow config value "([^"]*)" equals "([^"]*)"$`, s.theWorkflowConfigValueEquals)
	sc.Then(`^the workflow prompt contains "([^"]*)"$`, s.theWorkflowPromptContains)
	sc.Then(`^the workflow error code equals "([^"]*)"$`, s.theWorkflowErrorCodeEquals)
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		Name:                "workflow",
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format: "pretty",
			Paths:  []string{"../features"},
		},
	}
	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
