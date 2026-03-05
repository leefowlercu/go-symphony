package httpserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/leefowlercu/go-symphony/internal/domain"
)

type fakeOrchestrator struct {
	snapshot  domain.Snapshot
	issues    map[string]map[string]any
	refreshed bool
}

func (f *fakeOrchestrator) Snapshot() domain.Snapshot { return f.snapshot }
func (f *fakeOrchestrator) IssueDetails(identifier string) (map[string]any, bool) {
	v, ok := f.issues[identifier]
	return v, ok
}
func (f *fakeOrchestrator) RequestRefresh() { f.refreshed = true }

func testServer(orchestrator *fakeOrchestrator) *Server {
	return &Server{orchestrator: orchestrator}
}

func TestHandleStateReturnsSnapshot(t *testing.T) {
	f := &fakeOrchestrator{snapshot: domain.Snapshot{GeneratedAt: time.Now().UTC(), Running: []domain.RunningRow{{IssueIdentifier: "ABC-1"}}}}
	s := testServer(f)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/state", nil)
	res := httptest.NewRecorder()
	s.handleState(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	counts, _ := body["counts"].(map[string]any)
	if counts["running"].(float64) != 1 {
		t.Fatalf("expected running=1, got %#v", counts)
	}
}

func TestHandleIssueNotFound(t *testing.T) {
	s := testServer(&fakeOrchestrator{issues: map[string]map[string]any{}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/UNKNOWN", nil)
	res := httptest.NewRecorder()
	s.handleIssue(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.Code)
	}
}

func TestHandleRefreshQueuesTick(t *testing.T) {
	f := &fakeOrchestrator{}
	s := testServer(f)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/refresh", nil)
	res := httptest.NewRecorder()
	s.handleRefresh(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", res.Code)
	}
	if !f.refreshed {
		t.Fatal("expected refresh to be queued")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	s := testServer(&fakeOrchestrator{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/state", nil)
	res := httptest.NewRecorder()
	s.handleState(res, req)
	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", res.Code)
	}
	if got := res.Header().Get("Allow"); got != "GET" {
		t.Fatalf("expected Allow GET header, got %q", got)
	}
}

func TestHandleDashboard(t *testing.T) {
	s := testServer(&fakeOrchestrator{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	s.handleDashboard(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if _, err := ioutil.ReadAll(res.Body); err != nil {
		t.Fatalf("read response: %v", err)
	}
}
