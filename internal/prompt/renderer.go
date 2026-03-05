package prompt

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/leefowlercu/go-symphony/internal/domain"
	"github.com/leefowlercu/go-symphony/internal/workflow"
)

var (
	exprRe       = regexp.MustCompile(`\{\{\s*(.*?)\s*\}\}`)
	forVarRe     = regexp.MustCompile(`\{%\s*for\s+([A-Za-z_][A-Za-z0-9_]*)\s+in\s+.*?%\}`)
	identifierRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)
)

func Render(templateText string, issue domain.Issue, attempt *int) (string, error) {
	if strings.TrimSpace(templateText) == "" {
		templateText = "You are working on an issue from Linear."
	}
	issueMap, err := issueToMap(issue)
	if err != nil {
		return "", &workflow.Error{Code: workflow.ErrTemplateRender, Err: err}
	}
	if err := strictValidateTemplate(templateText, issueMap); err != nil {
		return "", &workflow.Error{Code: workflow.ErrTemplateRender, Err: err}
	}
	tpl, err := pongo2.FromString(templateText)
	if err != nil {
		return "", &workflow.Error{Code: workflow.ErrTemplateParse, Err: err}
	}
	ctx := pongo2.Context{
		"issue":   issueMap,
		"attempt": attempt,
	}
	out, err := tpl.Execute(ctx)
	if err != nil {
		return "", &workflow.Error{Code: workflow.ErrTemplateRender, Err: err}
	}
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("empty rendered prompt")
	}
	return out, nil
}

func strictValidateTemplate(templateText string, issue map[string]any) error {
	locals := map[string]struct{}{}
	for _, match := range forVarRe.FindAllStringSubmatch(templateText, -1) {
		if len(match) < 2 {
			continue
		}
		locals[match[1]] = struct{}{}
	}
	allowedRoots := map[string]struct{}{
		"issue":   {},
		"attempt": {},
		"true":    {},
		"false":   {},
		"nil":     {},
		"null":    {},
	}
	for key := range locals {
		allowedRoots[key] = struct{}{}
	}

	exprs := exprRe.FindAllStringSubmatch(templateText, -1)
	for _, match := range exprs {
		if len(match) < 2 {
			continue
		}
		raw := strings.TrimSpace(match[1])
		if raw == "" {
			continue
		}
		parts := splitFilterParts(raw)
		base := strings.TrimSpace(parts[0])
		if err := validateVariable(base, allowedRoots, issue); err != nil {
			return err
		}
		for _, filterExpr := range parts[1:] {
			name := parseFilterName(filterExpr)
			if name == "" {
				continue
			}
			if !pongo2.FilterExists(name) {
				return fmt.Errorf("unknown filter: %s", name)
			}
		}
	}
	return nil
}

func splitFilterParts(expr string) []string {
	parts := strings.Split(expr, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{expr}
	}
	return out
}

func parseFilterName(expr string) string {
	clean := strings.TrimSpace(expr)
	if clean == "" {
		return ""
	}
	if idx := strings.Index(clean, ":"); idx >= 0 {
		clean = strings.TrimSpace(clean[:idx])
	}
	if idx := strings.Index(clean, "("); idx >= 0 {
		clean = strings.TrimSpace(clean[:idx])
	}
	return clean
}

func validateVariable(expr string, allowed map[string]struct{}, issue map[string]any) error {
	if expr == "" {
		return nil
	}
	switch expr[0] {
	case '\'', '"':
		return nil
	}
	if strings.HasPrefix(expr, "[") || strings.HasPrefix(expr, "{") {
		return nil
	}
	root := identifierRe.FindString(expr)
	if root == "" {
		return nil
	}
	if _, ok := allowed[root]; !ok {
		return fmt.Errorf("unknown variable: %s", root)
	}
	if root != "issue" {
		return nil
	}
	path := strings.TrimPrefix(expr, "issue")
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if !strings.HasPrefix(path, ".") {
		return nil
	}
	keys := strings.Split(strings.TrimPrefix(path, "."), ".")
	var current any = issue
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil
		}
		m, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("unknown variable: issue.%s", strings.Join(keys, "."))
		}
		next, ok := m[key]
		if !ok {
			return fmt.Errorf("unknown variable: issue.%s", strings.Join(keys, "."))
		}
		current = next
	}
	return nil
}

func issueToMap(issue domain.Issue) (map[string]any, error) {
	b, err := json.Marshal(issue)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
