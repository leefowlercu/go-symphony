package subcommands

import (
	"fmt"
	"os"
	"path/filepath"
)

func ValidateWorkflowArgCount(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("expected at most one workflow path argument")
	}
	return nil
}

func ResolveWorkflowPath(args []string) (string, error) {
	path := "./WORKFLOW.md"
	if len(args) == 1 {
		path = args[0]
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve workflow path; %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("workflow file not found: %s", abs)
		}
		return "", fmt.Errorf("failed to stat workflow file; %w", err)
	}
	return abs, nil
}

func ResolvePort(cliPort int, workflowPort int, cliPortChanged bool) int {
	if cliPortChanged {
		return cliPort
	}
	if workflowPort >= 0 {
		return workflowPort
	}
	return 0
}
