package orchestrator

import (
	"context"
	"sync"
	"time"

	"github.com/leefowlercu/go-symphony/internal/domain"
)

type runningEntry struct {
	issue              domain.Issue
	identifier         string
	attempt            int
	startedAt          time.Time
	sessionID          string
	turnCount          int
	lastEvent          string
	lastMessage        string
	lastEventAt        *time.Time
	lastCodexTime      *time.Time
	inputTokens        int64
	outputTokens       int64
	totalTokens        int64
	lastReportedInput  int64
	lastReportedOutput int64
	lastReportedTotal  int64
	rateLimits         map[string]any
	cancel             context.CancelFunc
	releaseOnExit      bool
	cleanupOnExit      bool
	stopping           bool
}

type retryTimerEntry struct {
	entry domain.RetryEntry
	timer *time.Timer
}

type runtimeState struct {
	running             map[string]*runningEntry
	claimed             map[string]struct{}
	retry               map[string]*retryTimerEntry
	completed           map[string]struct{}
	codexTotals         domain.CodexTotals
	endedRuntimeSeconds float64
	latestRateLimits    map[string]any
}

func newRuntimeState() runtimeState {
	return runtimeState{
		running:   map[string]*runningEntry{},
		claimed:   map[string]struct{}{},
		retry:     map[string]*retryTimerEntry{},
		completed: map[string]struct{}{},
	}
}

type issueDetails struct {
	IssueIdentifier string           `json:"issue_identifier"`
	IssueID         string           `json:"issue_id"`
	Status          string           `json:"status"`
	Workspace       map[string]any   `json:"workspace"`
	Attempts        map[string]any   `json:"attempts"`
	Running         map[string]any   `json:"running"`
	Retry           map[string]any   `json:"retry"`
	LastError       any              `json:"last_error"`
	Tracked         map[string]any   `json:"tracked"`
	RecentEvents    []map[string]any `json:"recent_events"`
}

type snapshotStore struct {
	mu      sync.RWMutex
	snap    domain.Snapshot
	details map[string]issueDetails
}

func (s *snapshotStore) set(snapshot domain.Snapshot, details map[string]issueDetails) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap = snapshot
	s.details = details
}

func (s *snapshotStore) get() domain.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
}

func (s *snapshotStore) getIssue(identifier string) (issueDetails, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.details[identifier]
	return d, ok
}
