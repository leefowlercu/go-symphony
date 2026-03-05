package orchestrator

import (
	"math"
	"time"

	"github.com/leefowlercu/go-symphony/internal/config"
	"github.com/leefowlercu/go-symphony/internal/domain"
)

func (o *Orchestrator) scheduleRetry(issueID string, identifier string, attempt int, reason *string, continuation bool) {
	if existing, ok := o.state.retry[issueID]; ok {
		existing.timer.Stop()
		delete(o.state.retry, issueID)
	}

	cfg := config.Get()
	delay := 1 * time.Second
	if !continuation {
		exponent := math.Pow(2, float64(attempt-1))
		raw := float64(10_000) * exponent
		if raw > float64(cfg.Agent.MaxRetryBackoffMS) {
			raw = float64(cfg.Agent.MaxRetryBackoffMS)
		}
		delay = time.Duration(raw) * time.Millisecond
	}
	due := time.Now().UTC().Add(delay)
	entry := domain.RetryEntry{IssueID: issueID, IssueIdentifier: identifier, Attempt: attempt, DueAt: due, Error: reason}
	timer := time.AfterFunc(delay, func() {
		select {
		case o.retryFireCh <- issueID:
		default:
		}
	})
	o.state.retry[issueID] = &retryTimerEntry{entry: entry, timer: timer}
}

func (o *Orchestrator) clearRetry(issueID string) {
	if existing, ok := o.state.retry[issueID]; ok {
		existing.timer.Stop()
		delete(o.state.retry, issueID)
	}
}
