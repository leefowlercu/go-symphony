package linear

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/leefowlercu/go-symphony/internal/domain"
)

func normalizeIssue(node map[string]any) (domain.Issue, bool) {
	id, _ := node["id"].(string)
	identifier, _ := node["identifier"].(string)
	title, _ := node["title"].(string)
	state := nestedString(node, "state", "name")
	if strings.TrimSpace(id) == "" || strings.TrimSpace(identifier) == "" || strings.TrimSpace(title) == "" || strings.TrimSpace(state) == "" {
		return domain.Issue{}, false
	}

	var description *string
	if s, ok := node["description"].(string); ok {
		description = &s
	}
	var branchName *string
	if s, ok := node["branchName"].(string); ok {
		branchName = &s
	}
	var url *string
	if s, ok := node["url"].(string); ok {
		url = &s
	}
	priority := parsePriority(node["priority"])
	labels := parseLabels(node)
	blockers := parseBlockers(node)
	createdAt := parseISOTime(node["createdAt"])
	updatedAt := parseISOTime(node["updatedAt"])

	return domain.Issue{
		ID:          id,
		Identifier:  identifier,
		Title:       title,
		Description: description,
		Priority:    priority,
		State:       state,
		BranchName:  branchName,
		URL:         url,
		Labels:      labels,
		BlockedBy:   blockers,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, true
}

func parsePriority(v any) *int {
	switch t := v.(type) {
	case int:
		vv := t
		return &vv
	case int64:
		vv := int(t)
		return &vv
	case float64:
		vv := int(t)
		return &vv
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return nil
		}
		return &n
	default:
		return nil
	}
}

func parseLabels(node map[string]any) []string {
	labelsNode := nestedMap(node, "labels")
	nodes := nestedSlice(labelsNode, "nodes")
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		m, ok := n.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	return out
}

func parseBlockers(node map[string]any) []domain.BlockerRef {
	relations := nestedMap(node, "inverseRelations")
	nodes := nestedSlice(relations, "nodes")
	out := make([]domain.BlockerRef, 0)
	for _, n := range nodes {
		m, ok := n.(map[string]any)
		if !ok {
			continue
		}
		typeName, _ := m["type"].(string)
		if strings.ToLower(strings.TrimSpace(typeName)) != "blocks" {
			continue
		}
		issue := nestedMap(m, "issue")
		var ref domain.BlockerRef
		if v, ok := issue["id"].(string); ok && v != "" {
			ref.ID = &v
		}
		if v, ok := issue["identifier"].(string); ok && v != "" {
			ref.Identifier = &v
		}
		if v := nestedString(issue, "state", "name"); v != "" {
			ref.State = &v
		}
		out = append(out, ref)
	}
	return out
}

func parseISOTime(v any) *time.Time {
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func nestedMap(node map[string]any, key string) map[string]any {
	if node == nil {
		return nil
	}
	m, _ := node[key].(map[string]any)
	return m
}

func nestedSlice(node map[string]any, key string) []any {
	if node == nil {
		return nil
	}
	s, _ := node[key].([]any)
	return s
}

func nestedString(node map[string]any, keys ...string) string {
	curr := any(node)
	for _, key := range keys {
		m, ok := curr.(map[string]any)
		if !ok {
			return ""
		}
		curr = m[key]
	}
	s, _ := curr.(string)
	return s
}

func errf(code string, format string, args ...any) error {
	return fmt.Errorf("%s: %s", code, fmt.Sprintf(format, args...))
}
