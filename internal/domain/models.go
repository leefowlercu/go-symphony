package domain

import (
	"context"
	"time"
)

type Issue struct {
	ID          string       `json:"id"`
	Identifier  string       `json:"identifier"`
	Title       string       `json:"title"`
	Description *string      `json:"description,omitempty"`
	Priority    *int         `json:"priority,omitempty"`
	State       string       `json:"state"`
	BranchName  *string      `json:"branch_name,omitempty"`
	URL         *string      `json:"url,omitempty"`
	Labels      []string     `json:"labels"`
	BlockedBy   []BlockerRef `json:"blocked_by"`
	CreatedAt   *time.Time   `json:"created_at,omitempty"`
	UpdatedAt   *time.Time   `json:"updated_at,omitempty"`
}

type BlockerRef struct {
	ID         *string `json:"id,omitempty"`
	Identifier *string `json:"identifier,omitempty"`
	State      *string `json:"state,omitempty"`
}

type Workspace struct {
	Path         string `json:"path"`
	WorkspaceKey string `json:"workspace_key"`
	CreatedNow   bool   `json:"created_now"`
}

type RunAttempt struct {
	IssueID         string    `json:"issue_id"`
	IssueIdentifier string    `json:"issue_identifier"`
	Attempt         *int      `json:"attempt,omitempty"`
	WorkspacePath   string    `json:"workspace_path"`
	StartedAt       time.Time `json:"started_at"`
	Status          string    `json:"status"`
	Error           *string   `json:"error,omitempty"`
}

type RetryEntry struct {
	IssueID         string    `json:"issue_id"`
	IssueIdentifier string    `json:"issue_identifier"`
	Attempt         int       `json:"attempt"`
	DueAt           time.Time `json:"due_at"`
	Error           *string   `json:"error,omitempty"`
}

type LiveSession struct {
	Issue         Issue          `json:"issue"`
	SessionID     string         `json:"session_id"`
	TurnCount     int            `json:"turn_count"`
	StartedAt     time.Time      `json:"started_at"`
	LastEvent     string         `json:"last_event"`
	LastMessage   string         `json:"last_message"`
	LastEventAt   *time.Time     `json:"last_event_at,omitempty"`
	LastCodexAt   *time.Time     `json:"last_codex_timestamp,omitempty"`
	RetryAttempt  int            `json:"retry_attempt"`
	WorkspacePath string         `json:"workspace_path"`
	InputTokens   int64          `json:"input_tokens"`
	OutputTokens  int64          `json:"output_tokens"`
	TotalTokens   int64          `json:"total_tokens"`
	RateLimits    map[string]any `json:"rate_limits,omitempty"`
}

type CodexTotals struct {
	InputTokens    int64   `json:"input_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
	TotalTokens    int64   `json:"total_tokens"`
	SecondsRunning float64 `json:"seconds_running"`
}

type RunningRow struct {
	IssueID         string          `json:"issue_id"`
	IssueIdentifier string          `json:"issue_identifier"`
	State           string          `json:"state"`
	SessionID       string          `json:"session_id"`
	TurnCount       int             `json:"turn_count"`
	LastEvent       string          `json:"last_event"`
	LastMessage     string          `json:"last_message"`
	StartedAt       time.Time       `json:"started_at"`
	LastEventAt     *time.Time      `json:"last_event_at,omitempty"`
	Tokens          CodexTokenUsage `json:"tokens"`
}

type RetryRow struct {
	IssueID         string    `json:"issue_id"`
	IssueIdentifier string    `json:"issue_identifier"`
	Attempt         int       `json:"attempt"`
	DueAt           time.Time `json:"due_at"`
	Error           *string   `json:"error,omitempty"`
}

type CodexTokenUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

type Snapshot struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Running     []RunningRow   `json:"running"`
	Retrying    []RetryRow     `json:"retrying"`
	CodexTotals CodexTotals    `json:"codex_totals"`
	RateLimits  map[string]any `json:"rate_limits"`
}

type TrackerClient interface {
	FetchCandidateIssues(ctx context.Context) ([]Issue, error)
	FetchIssuesByStates(ctx context.Context, states []string) ([]Issue, error)
	FetchIssueStatesByIDs(ctx context.Context, ids []string) ([]Issue, error)
}

type AgentEvent struct {
	Event      string
	Timestamp  time.Time
	SessionID  string
	Message    string
	Usage      map[string]any
	RateLimits map[string]any
	Payload    map[string]any
}

type AttemptRequest struct {
	Issue   Issue
	Attempt *int
}

type AttemptResult struct {
	SessionID string
	Err       error
	StoppedBy string
}

type AgentClient interface {
	RunAttempt(ctx context.Context, req AttemptRequest, onEvent func(AgentEvent)) AttemptResult
}

type WorkspaceManager interface {
	EnsureWorkspace(ctx context.Context, issueIdentifier string) (Workspace, error)
	RemoveWorkspace(ctx context.Context, issueIdentifier string) error
}
