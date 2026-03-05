package runner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/leefowlercu/go-symphony/internal/codex/appserver"
	"github.com/leefowlercu/go-symphony/internal/codex/tools"
	"github.com/leefowlercu/go-symphony/internal/config"
	"github.com/leefowlercu/go-symphony/internal/domain"
	"github.com/leefowlercu/go-symphony/internal/prompt"
	"github.com/leefowlercu/go-symphony/internal/workspace"
)

type IssueStateFetcher interface {
	FetchIssueStatesByIDs(ctx context.Context, ids []string) ([]domain.Issue, error)
}

type AgentRunner struct {
	logger      *slog.Logger
	workspace   *workspace.Manager
	fetcher     IssueStateFetcher
	linearGraph tools.LinearGraphQLExecutor
}

func New(logger *slog.Logger, workspaceManager *workspace.Manager, fetcher IssueStateFetcher, linearGraph tools.LinearGraphQLExecutor) *AgentRunner {
	return &AgentRunner{logger: logger, workspace: workspaceManager, fetcher: fetcher, linearGraph: linearGraph}
}

func (r *AgentRunner) RunAttempt(ctx context.Context, req domain.AttemptRequest, onEvent func(domain.AgentEvent)) domain.AttemptResult {
	cfg := config.Get()
	issue := req.Issue

	ws, err := r.workspace.EnsureWorkspace(ctx, issue.Identifier)
	if err != nil {
		return domain.AttemptResult{Err: fmt.Errorf("workspace error; %w", err)}
	}
	if err := r.workspace.RunBeforeRun(ctx, ws.Path); err != nil {
		return domain.AttemptResult{Err: fmt.Errorf("before_run hook error; %w", err)}
	}
	defer r.workspace.RunAfterRun(context.Background(), ws.Path)

	toolManager := tools.NewDynamicToolManager(r.linearGraph)
	factory := appserver.Factory{Logger: r.logger, Settings: cfg.Codex, ToolManager: toolManager}
	client := factory.New(ws.Path, issue)
	defer client.Stop()

	if err := client.Start(ctx); err != nil {
		return domain.AttemptResult{Err: fmt.Errorf("agent session startup error; %w", err)}
	}

	maxTurns := cfg.Agent.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 1
	}

	current := issue
	for turn := 1; turn <= maxTurns; turn++ {
		turnPrompt, err := r.buildTurnPrompt(cfg.PromptTemplate, current, req.Attempt, turn, maxTurns)
		if err != nil {
			return domain.AttemptResult{Err: fmt.Errorf("prompt error; %w", err)}
		}
		sessionID, err := client.RunTurn(ctx, turnPrompt, onEvent)
		if err != nil {
			return domain.AttemptResult{SessionID: sessionID, Err: fmt.Errorf("agent turn error; %w", err)}
		}

		refreshed, err := r.fetcher.FetchIssueStatesByIDs(ctx, []string{current.ID})
		if err != nil {
			return domain.AttemptResult{SessionID: sessionID, Err: fmt.Errorf("issue state refresh error; %w", err)}
		}
		if len(refreshed) > 0 {
			current = refreshed[0]
		}
		if !isActiveState(current.State, cfg.Tracker.ActiveStates) {
			return domain.AttemptResult{SessionID: sessionID}
		}
	}
	return domain.AttemptResult{}
}

func (r *AgentRunner) buildTurnPrompt(tpl string, issue domain.Issue, attempt *int, turn int, maxTurns int) (string, error) {
	if turn == 1 {
		return prompt.Render(tpl, issue, attempt)
	}
	return fmt.Sprintf("Continue working on issue %s (%s). This is continuation turn %d of %d for this run. Do not repeat the original full task prompt; continue from existing thread history.", issue.Identifier, issue.Title, turn, maxTurns), nil
}

func isActiveState(state string, active []string) bool {
	s := strings.ToLower(strings.TrimSpace(state))
	for _, cand := range active {
		if s == strings.ToLower(strings.TrimSpace(cand)) {
			return true
		}
	}
	return false
}
