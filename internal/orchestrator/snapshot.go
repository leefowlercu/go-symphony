package orchestrator

import (
	"sort"
	"time"

	"github.com/leefowlercu/go-symphony/internal/domain"
)

func (o *Orchestrator) publishSnapshot(now time.Time) {
	runningRows := make([]domain.RunningRow, 0, len(o.state.running))
	details := map[string]issueDetails{}
	for issueID, entry := range o.state.running {
		row := domain.RunningRow{
			IssueID:         issueID,
			IssueIdentifier: entry.identifier,
			State:           entry.issue.State,
			SessionID:       entry.sessionID,
			TurnCount:       entry.turnCount,
			LastEvent:       entry.lastEvent,
			LastMessage:     entry.lastMessage,
			StartedAt:       entry.startedAt,
			LastEventAt:     entry.lastEventAt,
			Tokens: domain.CodexTokenUsage{
				InputTokens:  entry.inputTokens,
				OutputTokens: entry.outputTokens,
				TotalTokens:  entry.totalTokens,
			},
		}
		runningRows = append(runningRows, row)
		details[entry.identifier] = issueDetails{
			IssueIdentifier: entry.identifier,
			IssueID:         issueID,
			Status:          "running",
			Workspace:       map[string]any{"path": entry.issue.Identifier},
			Attempts:        map[string]any{"current_retry_attempt": entry.attempt},
			Running: map[string]any{
				"session_id":    entry.sessionID,
				"turn_count":    entry.turnCount,
				"state":         entry.issue.State,
				"started_at":    entry.startedAt,
				"last_event":    entry.lastEvent,
				"last_message":  entry.lastMessage,
				"last_event_at": entry.lastEventAt,
				"tokens":        map[string]any{"input_tokens": entry.inputTokens, "output_tokens": entry.outputTokens, "total_tokens": entry.totalTokens},
			},
			Retry:        nil,
			Tracked:      map[string]any{},
			RecentEvents: []map[string]any{},
		}
	}
	sort.Slice(runningRows, func(i, j int) bool { return runningRows[i].IssueIdentifier < runningRows[j].IssueIdentifier })

	retryRows := make([]domain.RetryRow, 0, len(o.state.retry))
	for issueID, retry := range o.state.retry {
		retryRows = append(retryRows, domain.RetryRow{
			IssueID:         issueID,
			IssueIdentifier: retry.entry.IssueIdentifier,
			Attempt:         retry.entry.Attempt,
			DueAt:           retry.entry.DueAt,
			Error:           retry.entry.Error,
		})
		if _, ok := details[retry.entry.IssueIdentifier]; !ok {
			details[retry.entry.IssueIdentifier] = issueDetails{
				IssueIdentifier: retry.entry.IssueIdentifier,
				IssueID:         issueID,
				Status:          "retrying",
				Workspace:       map[string]any{},
				Attempts:        map[string]any{"current_retry_attempt": retry.entry.Attempt},
				Running:         nil,
				Retry: map[string]any{
					"attempt": retry.entry.Attempt,
					"due_at":  retry.entry.DueAt,
					"error":   retry.entry.Error,
				},
				Tracked:      map[string]any{},
				RecentEvents: []map[string]any{},
			}
		}
	}
	sort.Slice(retryRows, func(i, j int) bool { return retryRows[i].IssueIdentifier < retryRows[j].IssueIdentifier })

	seconds := o.state.endedRuntimeSeconds
	for _, entry := range o.state.running {
		seconds += now.Sub(entry.startedAt).Seconds()
	}
	totals := o.state.codexTotals
	totals.SecondsRunning = seconds
	snapshot := domain.Snapshot{GeneratedAt: now.UTC(), Running: runningRows, Retrying: retryRows, CodexTotals: totals, RateLimits: o.state.latestRateLimits}
	o.snapshots.set(snapshot, details)
}
