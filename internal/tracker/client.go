package tracker

import (
	"context"

	"github.com/leefowlercu/go-symphony/internal/domain"
)

type Client interface {
	FetchCandidateIssues(ctx context.Context) ([]domain.Issue, error)
	FetchIssuesByStates(ctx context.Context, states []string) ([]domain.Issue, error)
	FetchIssueStatesByIDs(ctx context.Context, ids []string) ([]domain.Issue, error)
}
