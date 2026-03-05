package orchestrator

import (
	"testing"
	"time"

	"github.com/leefowlercu/go-symphony/internal/domain"
)

func TestSortForDispatchOrder(t *testing.T) {
	now := time.Now().UTC()
	p1 := 1
	p2 := 2
	issues := []domain.Issue{
		{ID: "3", Identifier: "ABC-3", Title: "t3", State: "Todo", Priority: &p2, CreatedAt: ptrTime(now.Add(2 * time.Hour))},
		{ID: "2", Identifier: "ABC-2", Title: "t2", State: "Todo", Priority: nil, CreatedAt: ptrTime(now.Add(-2 * time.Hour))},
		{ID: "1", Identifier: "ABC-1", Title: "t1", State: "Todo", Priority: &p1, CreatedAt: ptrTime(now.Add(-1 * time.Hour))},
	}
	sorted := sortForDispatch(issues)
	if sorted[0].Identifier != "ABC-1" || sorted[1].Identifier != "ABC-3" || sorted[2].Identifier != "ABC-2" {
		t.Fatalf("unexpected sort order: %#v", []string{sorted[0].Identifier, sorted[1].Identifier, sorted[2].Identifier})
	}
}

func TestStateSlotsAvailable(t *testing.T) {
	running := map[string]*runningEntry{
		"1": {issue: domain.Issue{State: "Todo"}},
	}
	limits := map[string]int{"todo": 1}
	if stateSlotsAvailable("Todo", limits, running, 10) {
		t.Fatal("expected no slots available for todo")
	}
	if !stateSlotsAvailable("In Progress", limits, running, 10) {
		t.Fatal("expected global fallback slots to be available")
	}
}

func TestParseTokenUsage(t *testing.T) {
	usage := map[string]any{"input_tokens": 12.0, "output_tokens": 8.0, "total_tokens": 20.0}
	in, out, total := parseTokenUsage(usage)
	if in != 12 || out != 8 || total != 20 {
		t.Fatalf("unexpected token parse: in=%d out=%d total=%d", in, out, total)
	}
}

func ptrTime(t time.Time) *time.Time {
	u := t.UTC()
	return &u
}
