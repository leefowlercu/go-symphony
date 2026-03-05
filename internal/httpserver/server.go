package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/leefowlercu/go-symphony/internal/domain"
)

type orchestratorAPI interface {
	Snapshot() domain.Snapshot
	IssueDetails(identifier string) (map[string]any, bool)
	RequestRefresh()
}

type Dependencies struct {
	Logger       *slog.Logger
	Port         int
	Orchestrator orchestratorAPI
}

type Server struct {
	logger       *slog.Logger
	port         int
	orchestrator orchestratorAPI
}

func New(deps Dependencies) *Server {
	return &Server{logger: deps.Logger, port: deps.Port, orchestrator: deps.Orchestrator}
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/api/v1/state", s.handleState)
	mux.HandleFunc("/api/v1/refresh", s.handleRefresh)
	mux.HandleFunc("/api/v1/", s.handleIssue)

	server := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server starting", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
