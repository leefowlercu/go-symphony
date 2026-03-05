package workflow

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Definition struct {
	Path           string
	Config         map[string]any
	PromptTemplate string
	ModTime        time.Time
}

func Load(path string) (Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Definition{}, &Error{Code: ErrMissingWorkflowFile, Err: err}
		}
		return Definition{}, &Error{Code: ErrWorkflowParse, Err: err}
	}

	stat, err := os.Stat(path)
	if err != nil {
		return Definition{}, &Error{Code: ErrWorkflowParse, Err: err}
	}

	cfg, prompt, err := parseWorkflow(string(data))
	if err != nil {
		return Definition{}, err
	}

	return Definition{Path: path, Config: cfg, PromptTemplate: prompt, ModTime: stat.ModTime().UTC()}, nil
}

func parseWorkflow(content string) (map[string]any, string, error) {
	trimmed := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(trimmed, "---\n") {
		return map[string]any{}, strings.TrimSpace(trimmed), nil
	}

	rest := strings.TrimPrefix(trimmed, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return nil, "", &Error{Code: ErrWorkflowParse, Err: fmt.Errorf("missing front matter terminator")}
	}

	front := rest[:idx]
	body := rest[idx+len("\n---\n"):]

	if strings.TrimSpace(front) == "" {
		return map[string]any{}, strings.TrimSpace(body), nil
	}

	var raw any
	if err := yaml.Unmarshal([]byte(front), &raw); err != nil {
		return nil, "", &Error{Code: ErrWorkflowParse, Err: err}
	}

	root, ok := raw.(map[string]any)
	if !ok {
		return nil, "", &Error{Code: ErrWorkflowFrontMatterType, Err: fmt.Errorf("front matter must decode to map")}
	}

	return root, strings.TrimSpace(body), nil
}
