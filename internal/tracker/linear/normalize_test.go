package linear

import "testing"

func TestNormalizeIssue(t *testing.T) {
	node := map[string]any{
		"id":          "1",
		"identifier":  "ABC-1",
		"title":       "Demo",
		"description": "Body",
		"priority":    float64(2),
		"state":       map[string]any{"name": "Todo"},
		"labels":      map[string]any{"nodes": []any{map[string]any{"name": "Backend"}}},
		"inverseRelations": map[string]any{"nodes": []any{
			map[string]any{"type": "blocks", "issue": map[string]any{"id": "b1", "identifier": "ABC-0", "state": map[string]any{"name": "In Progress"}}},
		}},
		"createdAt": "2026-01-01T00:00:00Z",
		"updatedAt": "2026-01-02T00:00:00Z",
	}
	issue, ok := normalizeIssue(node)
	if !ok {
		t.Fatal("expected normalized issue")
	}
	if issue.Identifier != "ABC-1" {
		t.Fatalf("unexpected identifier: %q", issue.Identifier)
	}
	if len(issue.Labels) != 1 || issue.Labels[0] != "backend" {
		t.Fatalf("expected lowercase labels, got %#v", issue.Labels)
	}
	if len(issue.BlockedBy) != 1 {
		t.Fatalf("expected blockers, got %#v", issue.BlockedBy)
	}
	if issue.Priority == nil || *issue.Priority != 2 {
		t.Fatalf("expected priority 2, got %#v", issue.Priority)
	}
}

func TestNormalizeIssueRequiresCoreFields(t *testing.T) {
	_, ok := normalizeIssue(map[string]any{"identifier": "ABC-1", "title": "Demo"})
	if ok {
		t.Fatal("expected invalid issue without id/state")
	}
}
