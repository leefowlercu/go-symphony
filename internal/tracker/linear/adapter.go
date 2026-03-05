package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/leefowlercu/go-symphony/internal/config"
	"github.com/leefowlercu/go-symphony/internal/domain"
)

const pageSize = 50

type Adapter struct {
	cfg    config.TrackerConfig
	logger *slog.Logger
	http   *http.Client
}

func NewAdapter(cfg config.TrackerConfig, logger *slog.Logger) *Adapter {
	return &Adapter{
		cfg:    cfg,
		logger: logger,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *Adapter) FetchCandidateIssues(ctx context.Context) ([]domain.Issue, error) {
	if strings.TrimSpace(a.cfg.APIKey) == "" {
		return nil, errf("missing_tracker_api_key", "api key missing")
	}
	if strings.TrimSpace(a.cfg.ProjectSlug) == "" {
		return nil, errf("missing_tracker_project_slug", "project slug missing")
	}
	issues := make([]domain.Issue, 0)
	var after any
	for {
		body, err := a.graphql(ctx, candidateQuery, map[string]any{
			"projectSlug": a.cfg.ProjectSlug,
			"stateNames":  a.cfg.ActiveStates,
			"first":       pageSize,
			"after":       after,
		})
		if err != nil {
			return nil, err
		}
		nodes, pageInfo, err := extractNodesAndPageInfo(body)
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			issue, ok := normalizeIssue(node)
			if ok {
				issues = append(issues, issue)
			}
		}
		hasNext, _ := pageInfo["hasNextPage"].(bool)
		if !hasNext {
			return issues, nil
		}
		endCursor, ok := pageInfo["endCursor"].(string)
		if !ok || strings.TrimSpace(endCursor) == "" {
			return nil, errf("linear_missing_end_cursor", "hasNextPage=true without endCursor")
		}
		after = endCursor
	}
}

func (a *Adapter) FetchIssuesByStates(ctx context.Context, states []string) ([]domain.Issue, error) {
	if len(states) == 0 {
		return []domain.Issue{}, nil
	}
	cfg := a.cfg
	cfg.ActiveStates = states
	clone := &Adapter{cfg: cfg, logger: a.logger, http: a.http}
	return clone.FetchCandidateIssues(ctx)
}

func (a *Adapter) FetchIssueStatesByIDs(ctx context.Context, ids []string) ([]domain.Issue, error) {
	if len(ids) == 0 {
		return []domain.Issue{}, nil
	}
	body, err := a.graphql(ctx, refreshByIDsQuery, map[string]any{
		"ids":   ids,
		"first": min(len(ids), pageSize),
	})
	if err != nil {
		return nil, err
	}
	issuesNode := nestedMap(nestedMap(body, "data"), "issues")
	nodesRaw, _ := issuesNode["nodes"].([]any)
	issues := make([]domain.Issue, 0, len(nodesRaw))
	for _, n := range nodesRaw {
		m, ok := n.(map[string]any)
		if !ok {
			continue
		}
		issue, ok := normalizeIssue(m)
		if ok {
			issues = append(issues, issue)
		}
	}
	return issues, nil
}

func (a *Adapter) GraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	return a.graphql(ctx, query, variables)
}

func (a *Adapter) graphql(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	if strings.TrimSpace(a.cfg.APIKey) == "" {
		return nil, errf("missing_tracker_api_key", "api key missing")
	}
	payload := map[string]any{"query": query, "variables": variables}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("linear_api_request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.Endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("linear_api_request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", a.cfg.APIKey)

	resp, err := a.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("linear_api_request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("linear_api_request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		a.logger.Error("linear request failed", "status", resp.StatusCode, "body", truncate(body, 1000))
		return nil, fmt.Errorf("linear_api_status: status=%d", resp.StatusCode)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("linear_unknown_payload: %w", err)
	}
	if errs, ok := decoded["errors"].([]any); ok && len(errs) > 0 {
		return nil, fmt.Errorf("linear_graphql_errors: %v", errs)
	}
	return decoded, nil
}

func extractNodesAndPageInfo(body map[string]any) ([]map[string]any, map[string]any, error) {
	issuesNode := nestedMap(nestedMap(body, "data"), "issues")
	if issuesNode == nil {
		return nil, nil, errf("linear_unknown_payload", "missing data.issues")
	}
	nodesRaw, _ := issuesNode["nodes"].([]any)
	nodes := make([]map[string]any, 0, len(nodesRaw))
	for _, n := range nodesRaw {
		m, ok := n.(map[string]any)
		if !ok {
			continue
		}
		nodes = append(nodes, m)
	}
	pageInfo, _ := issuesNode["pageInfo"].(map[string]any)
	if pageInfo == nil {
		return nil, nil, errf("linear_unknown_payload", "missing data.issues.pageInfo")
	}
	return nodes, pageInfo, nil
}

func truncate(in []byte, max int) string {
	s := strings.TrimSpace(strings.ReplaceAll(string(in), "\n", " "))
	if len(s) <= max {
		return s
	}
	return s[:max] + "...<truncated>"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
