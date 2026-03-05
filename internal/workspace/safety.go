package workspace

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var safeWorkspaceRe = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func sanitizeWorkspaceKey(identifier string) string {
	trimmed := strings.TrimSpace(identifier)
	if trimmed == "" {
		return "_"
	}
	return safeWorkspaceRe.ReplaceAllString(trimmed, "_")
}

func ensureWithinRoot(root string, candidate string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve workspace root; %w", err)
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return fmt.Errorf("resolve workspace path; %w", err)
	}
	rel, err := filepath.Rel(absRoot, absCandidate)
	if err != nil {
		return fmt.Errorf("rel workspace path; %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("workspace path outside root")
	}
	return nil
}
