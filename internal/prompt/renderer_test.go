package prompt

import (
	"testing"

	"github.com/leefowlercu/go-symphony/internal/domain"
)

func TestRenderRendersIssueAndAttempt(t *testing.T) {
	attempt := 2
	issue := domain.Issue{ID: "1", Identifier: "ABC-1", Title: "Demo", State: "Todo", Labels: []string{"backend"}}
	out, err := Render("Issue {{ issue.identifier }} attempt {{ attempt }}", issue, &attempt)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out != "Issue ABC-1 attempt 2" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRenderFailsUnknownVariable(t *testing.T) {
	issue := domain.Issue{ID: "1", Identifier: "ABC-1", Title: "Demo", State: "Todo"}
	if _, err := Render("Unknown {{ does_not_exist }}", issue, nil); err == nil {
		t.Fatal("expected unknown variable error")
	}
}

func TestRenderFailsUnknownIssuePath(t *testing.T) {
	issue := domain.Issue{ID: "1", Identifier: "ABC-1", Title: "Demo", State: "Todo"}
	if _, err := Render("Unknown {{ issue.not_real }}", issue, nil); err == nil {
		t.Fatal("expected unknown issue variable error")
	}
}

func TestRenderFailsUnknownFilter(t *testing.T) {
	issue := domain.Issue{ID: "1", Identifier: "ABC-1", Title: "Demo", State: "Todo"}
	if _, err := Render("Bad filter {{ issue.identifier|notafilter }}", issue, nil); err == nil {
		t.Fatal("expected unknown filter error")
	}
}
