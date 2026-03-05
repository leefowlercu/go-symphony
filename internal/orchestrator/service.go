package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/leefowlercu/go-symphony/internal/config"
	"github.com/leefowlercu/go-symphony/internal/domain"
	"github.com/leefowlercu/go-symphony/internal/runner"
	"github.com/leefowlercu/go-symphony/internal/workflow"
)

type Dependencies struct {
	Logger           *slog.Logger
	Tracker          domain.TrackerClient
	WorkspaceManager domain.WorkspaceManager
	AgentRunner      *runner.AgentRunner
}

type workerUpdate struct {
	IssueID string
	Event   domain.AgentEvent
}

type workerDone struct {
	IssueID string
	Result  domain.AttemptResult
}

type Orchestrator struct {
	logger    *slog.Logger
	tracker   domain.TrackerClient
	workspace domain.WorkspaceManager
	runner    *runner.AgentRunner

	workerUpdateCh chan workerUpdate
	workerDoneCh   chan workerDone
	retryFireCh    chan string
	refreshCh      chan struct{}
	reloadCh       chan struct{}

	state     runtimeState
	snapshots snapshotStore
}

func New(deps Dependencies) *Orchestrator {
	return &Orchestrator{
		logger:         deps.Logger,
		tracker:        deps.Tracker,
		workspace:      deps.WorkspaceManager,
		runner:         deps.AgentRunner,
		workerUpdateCh: make(chan workerUpdate, 1024),
		workerDoneCh:   make(chan workerDone, 256),
		retryFireCh:    make(chan string, 256),
		refreshCh:      make(chan struct{}, 1),
		reloadCh:       make(chan struct{}, 1),
		state:          newRuntimeState(),
		snapshots:      snapshotStore{details: map[string]issueDetails{}},
	}
}

func (o *Orchestrator) Snapshot() domain.Snapshot { return o.snapshots.get() }

func (o *Orchestrator) IssueDetails(identifier string) (map[string]any, bool) {
	d, ok := o.snapshots.getIssue(identifier)
	if !ok {
		return nil, false
	}
	return map[string]any{
		"issue_identifier": d.IssueIdentifier,
		"issue_id":         d.IssueID,
		"status":           d.Status,
		"workspace":        d.Workspace,
		"attempts":         d.Attempts,
		"running":          d.Running,
		"retry":            d.Retry,
		"last_error":       d.LastError,
		"tracked":          d.Tracked,
		"recent_events":    d.RecentEvents,
	}, true
}

func (o *Orchestrator) RequestRefresh() {
	select {
	case o.refreshCh <- struct{}{}:
	default:
	}
}

func (o *Orchestrator) Run(ctx context.Context) error {
	if err := config.ValidateDispatchConfig(); err != nil {
		return fmt.Errorf("startup validation failed; %w", err)
	}
	if err := o.startupCleanup(ctx); err != nil {
		o.logger.Warn("startup cleanup warning", "error", err)
	}

	w, err := workflow.NewWatcher(config.Get().WorkflowPath, func() {
		select {
		case o.reloadCh <- struct{}{}:
		default:
		}
	})
	if err == nil {
		go func() {
			if runErr := w.Run(ctx); runErr != nil {
				o.logger.Warn("workflow watch error", "error", runErr)
			}
		}()
	}

	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-o.reloadCh:
			if err := config.Reload(); err != nil {
				o.logger.Error("workflow reload failed; keeping last known good", "error", err)
			} else {
				o.logger.Info("workflow reloaded")
			}
		case <-o.refreshCh:
			o.tick(ctx)
			resetTimer(timer, time.Duration(config.Get().Polling.IntervalMS)*time.Millisecond)
		case <-timer.C:
			o.tick(ctx)
			resetTimer(timer, time.Duration(config.Get().Polling.IntervalMS)*time.Millisecond)
		case upd := <-o.workerUpdateCh:
			o.handleWorkerUpdate(upd)
			o.publishSnapshot(time.Now().UTC())
		case done := <-o.workerDoneCh:
			o.handleWorkerDone(done)
			o.publishSnapshot(time.Now().UTC())
		case issueID := <-o.retryFireCh:
			o.handleRetryTimer(ctx, issueID)
			o.publishSnapshot(time.Now().UTC())
		}
	}
}

func (o *Orchestrator) tick(ctx context.Context) {
	now := time.Now().UTC()
	config.ReloadIfStale()
	o.reconcileRunning(ctx, now)
	if err := config.ValidateDispatchConfig(); err != nil {
		o.logger.Error("dispatch validation failed", "error", err)
		o.publishSnapshot(now)
		return
	}

	issues, err := o.tracker.FetchCandidateIssues(ctx)
	if err != nil {
		o.logger.Error("candidate fetch failed", "error", err)
		o.publishSnapshot(now)
		return
	}
	sorted := sortForDispatch(issues)
	for _, issue := range sorted {
		if o.availableSlots() <= 0 {
			break
		}
		if !o.shouldDispatch(issue) {
			continue
		}
		o.dispatch(ctx, issue, 0, false)
	}
	o.publishSnapshot(now)
}

func (o *Orchestrator) startupCleanup(ctx context.Context) error {
	cfg := config.Get()
	issues, err := o.tracker.FetchIssuesByStates(ctx, cfg.Tracker.TerminalStates)
	if err != nil {
		return err
	}
	for _, issue := range issues {
		if err := o.workspace.RemoveWorkspace(ctx, issue.Identifier); err != nil {
			o.logger.Warn("startup terminal workspace cleanup failed", "issue_identifier", issue.Identifier, "error", err)
		}
	}
	return nil
}

func (o *Orchestrator) dispatch(ctx context.Context, issue domain.Issue, attempt int, fromRetry bool) {
	if _, ok := o.state.running[issue.ID]; ok {
		return
	}
	if _, ok := o.state.claimed[issue.ID]; ok && !fromRetry {
		return
	}

	workerCtx, cancel := context.WithCancel(ctx)
	entry := &runningEntry{issue: issue, identifier: issue.Identifier, attempt: attempt, startedAt: time.Now().UTC(), cancel: cancel}
	o.state.running[issue.ID] = entry
	o.state.claimed[issue.ID] = struct{}{}
	o.clearRetry(issue.ID)

	var attemptPtr *int
	if attempt > 0 {
		v := attempt
		attemptPtr = &v
	}
	go func(issue domain.Issue, attemptValue *int) {
		res := o.runner.RunAttempt(workerCtx, domain.AttemptRequest{Issue: issue, Attempt: attemptValue}, func(event domain.AgentEvent) {
			o.workerUpdateCh <- workerUpdate{IssueID: issue.ID, Event: event}
		})
		o.workerDoneCh <- workerDone{IssueID: issue.ID, Result: res}
	}(issue, attemptPtr)
}

func (o *Orchestrator) reconcileRunning(ctx context.Context, now time.Time) {
	cfg := config.Get()
	if cfg.Codex.StallTimeoutMS > 0 {
		for issueID, entry := range o.state.running {
			if entry.stopping {
				continue
			}
			reference := entry.startedAt
			if entry.lastCodexTime != nil {
				reference = *entry.lastCodexTime
			}
			if now.Sub(reference) > time.Duration(cfg.Codex.StallTimeoutMS)*time.Millisecond {
				entry.stopping = true
				entry.cancel()
				o.logger.Warn("stalled run detected", "issue_id", issueID, "issue_identifier", entry.identifier)
			}
		}
	}

	ids := make([]string, 0, len(o.state.running))
	for id := range o.state.running {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}
	states, err := o.tracker.FetchIssueStatesByIDs(ctx, ids)
	if err != nil {
		o.logger.Warn("state refresh failed; keeping workers running", "error", err)
		return
	}
	byID := map[string]domain.Issue{}
	for _, issue := range states {
		byID[issue.ID] = issue
	}
	for issueID, entry := range o.state.running {
		current, ok := byID[issueID]
		if !ok {
			continue
		}
		if isTerminalState(current.State, cfg.Tracker.TerminalStates) {
			entry.releaseOnExit = true
			entry.cleanupOnExit = true
			entry.stopping = true
			entry.cancel()
			continue
		}
		if !isActiveState(current.State, cfg.Tracker.ActiveStates) {
			entry.releaseOnExit = true
			entry.cleanupOnExit = false
			entry.stopping = true
			entry.cancel()
			continue
		}
		entry.issue = current
	}
}

func (o *Orchestrator) handleRetryTimer(ctx context.Context, issueID string) {
	entry, ok := o.state.retry[issueID]
	if !ok {
		return
	}
	delete(o.state.retry, issueID)

	issues, err := o.tracker.FetchCandidateIssues(ctx)
	if err != nil {
		reason := "retry poll failed"
		o.scheduleRetry(issueID, entry.entry.IssueIdentifier, entry.entry.Attempt+1, &reason, false)
		return
	}
	var found *domain.Issue
	for i := range issues {
		if issues[i].ID == issueID {
			found = &issues[i]
			break
		}
	}
	if found == nil {
		delete(o.state.claimed, issueID)
		return
	}
	if o.availableSlots() <= 0 || !o.shouldDispatchRetry(*found) {
		reason := "no available orchestrator slots"
		o.scheduleRetry(issueID, found.Identifier, entry.entry.Attempt+1, &reason, false)
		return
	}
	o.dispatch(ctx, *found, entry.entry.Attempt, true)
}

func (o *Orchestrator) handleWorkerUpdate(upd workerUpdate) {
	entry, ok := o.state.running[upd.IssueID]
	if !ok {
		return
	}
	event := upd.Event
	entry.lastEvent = event.Event
	entry.lastMessage = event.Message
	now := event.Timestamp
	if now.IsZero() {
		now = time.Now().UTC()
	}
	entry.lastEventAt = &now
	entry.lastCodexTime = &now
	if event.SessionID != "" {
		entry.sessionID = event.SessionID
	}
	if event.Event == "turn_completed" {
		entry.turnCount++
	}
	if event.RateLimits != nil {
		entry.rateLimits = event.RateLimits
		o.state.latestRateLimits = event.RateLimits
	}
	if event.Usage != nil {
		in, out, total := parseTokenUsage(event.Usage)
		deltaIn := in - entry.lastReportedInput
		deltaOut := out - entry.lastReportedOutput
		deltaTotal := total - entry.lastReportedTotal
		if deltaIn > 0 {
			o.state.codexTotals.InputTokens += deltaIn
			entry.inputTokens += deltaIn
		}
		if deltaOut > 0 {
			o.state.codexTotals.OutputTokens += deltaOut
			entry.outputTokens += deltaOut
		}
		if deltaTotal > 0 {
			o.state.codexTotals.TotalTokens += deltaTotal
			entry.totalTokens += deltaTotal
		}
		entry.lastReportedInput = in
		entry.lastReportedOutput = out
		entry.lastReportedTotal = total
	}
}

func (o *Orchestrator) handleWorkerDone(done workerDone) {
	entry, ok := o.state.running[done.IssueID]
	if !ok {
		return
	}
	delete(o.state.running, done.IssueID)
	o.state.endedRuntimeSeconds += time.Since(entry.startedAt).Seconds()
	if done.Result.SessionID != "" {
		entry.sessionID = done.Result.SessionID
	}
	if entry.cleanupOnExit {
		_ = o.workspace.RemoveWorkspace(context.Background(), entry.identifier)
	}
	if entry.releaseOnExit {
		delete(o.state.claimed, done.IssueID)
		return
	}
	if done.Result.Err == nil {
		o.state.completed[done.IssueID] = struct{}{}
		o.scheduleRetry(done.IssueID, entry.identifier, 1, nil, true)
		return
	}
	next := entry.attempt + 1
	if next <= 0 {
		next = 1
	}
	reason := done.Result.Err.Error()
	o.scheduleRetry(done.IssueID, entry.identifier, next, &reason, false)
}

func (o *Orchestrator) availableSlots() int {
	cfg := config.Get()
	slots := cfg.Agent.MaxConcurrentAgents - len(o.state.running)
	if slots < 0 {
		return 0
	}
	return slots
}

func (o *Orchestrator) shouldDispatch(issue domain.Issue) bool {
	return o.shouldDispatchWithClaimMode(issue, false)
}

func (o *Orchestrator) shouldDispatchRetry(issue domain.Issue) bool {
	return o.shouldDispatchWithClaimMode(issue, true)
}

func (o *Orchestrator) shouldDispatchWithClaimMode(issue domain.Issue, allowClaimed bool) bool {
	cfg := config.Get()
	if strings.TrimSpace(issue.ID) == "" || strings.TrimSpace(issue.Identifier) == "" || strings.TrimSpace(issue.Title) == "" || strings.TrimSpace(issue.State) == "" {
		return false
	}
	if !isActiveState(issue.State, cfg.Tracker.ActiveStates) || isTerminalState(issue.State, cfg.Tracker.TerminalStates) {
		return false
	}
	if _, ok := o.state.running[issue.ID]; ok {
		return false
	}
	if _, ok := o.state.claimed[issue.ID]; ok && !allowClaimed {
		return false
	}
	if o.availableSlots() <= 0 {
		return false
	}
	if !stateSlotsAvailable(issue.State, cfg.Agent.MaxConcurrentAgentsByState, o.state.running, cfg.Agent.MaxConcurrentAgents) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(issue.State), "todo") {
		for _, blocker := range issue.BlockedBy {
			if blocker.State == nil {
				continue
			}
			if !isTerminalState(*blocker.State, cfg.Tracker.TerminalStates) {
				return false
			}
		}
	}
	return true
}

func sortForDispatch(issues []domain.Issue) []domain.Issue {
	out := append([]domain.Issue(nil), issues...)
	sort.SliceStable(out, func(i, j int) bool {
		pi := priorityValue(out[i].Priority)
		pj := priorityValue(out[j].Priority)
		if pi != pj {
			return pi < pj
		}
		ti := parseSortableTime(out[i].CreatedAt)
		tj := parseSortableTime(out[j].CreatedAt)
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return out[i].Identifier < out[j].Identifier
	})
	return out
}

func stateSlotsAvailable(state string, limits map[string]int, running map[string]*runningEntry, global int) bool {
	key := strings.ToLower(strings.TrimSpace(state))
	limit, ok := limits[key]
	if !ok {
		limit = global
	}
	count := 0
	for _, e := range running {
		if strings.ToLower(strings.TrimSpace(e.issue.State)) == key {
			count++
		}
	}
	return count < limit
}

func parseTokenUsage(usage map[string]any) (int64, int64, int64) {
	if usage == nil {
		return 0, 0, 0
	}
	in := extractInt64Any(usage, "input_tokens", "inputTokens")
	out := extractInt64Any(usage, "output_tokens", "outputTokens")
	total := extractInt64Any(usage, "total_tokens", "totalTokens")
	if total == 0 {
		total = in + out
	}
	return in, out, total
}

func extractInt64Any(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case int64:
				return t
			case int:
				return int64(t)
			case float64:
				return int64(t)
			}
		}
	}
	for _, v := range m {
		if nested, ok := v.(map[string]any); ok {
			if n := extractInt64Any(nested, keys...); n > 0 {
				return n
			}
		}
	}
	return 0
}

func isActiveState(state string, active []string) bool {
	s := strings.ToLower(strings.TrimSpace(state))
	for _, a := range active {
		if s == strings.ToLower(strings.TrimSpace(a)) {
			return true
		}
	}
	return false
}

func isTerminalState(state string, terminal []string) bool {
	s := strings.ToLower(strings.TrimSpace(state))
	for _, t := range terminal {
		if s == strings.ToLower(strings.TrimSpace(t)) {
			return true
		}
	}
	return false
}

func priorityValue(p *int) int {
	if p == nil {
		return 999999
	}
	return *p
}

func parseSortableTime(t *time.Time) time.Time {
	if t == nil {
		return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return t.UTC()
}

func resetTimer(timer *time.Timer, d time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(d)
}
