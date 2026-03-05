package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/leefowlercu/go-symphony/cmd/run/subcommands"
	"github.com/leefowlercu/go-symphony/internal/config"
	"github.com/leefowlercu/go-symphony/internal/httpserver"
	"github.com/leefowlercu/go-symphony/internal/logging"
	"github.com/leefowlercu/go-symphony/internal/orchestrator"
	"github.com/leefowlercu/go-symphony/internal/runner"
	"github.com/leefowlercu/go-symphony/internal/tracker/linear"
	"github.com/leefowlercu/go-symphony/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	runPort         int
	runPortExplicit bool
	runLogsRoot     string
)

var RunCmd = &cobra.Command{
	Use:   "run [path-to-workflow-md]",
	Short: "Run Symphony scheduler service loop",
	Long: "Run Symphony scheduler service loop for tracker-driven issue execution.\n\n" +
		"The command loads WORKFLOW.md configuration, starts reconciliation and dispatch loops, " +
		"and optionally serves observability endpoints.",
	Example: "# Run with default workflow path\n" +
		"  symphony run\n\n" +
		"# Run with explicit workflow path\n" +
		"  symphony run /path/to/WORKFLOW.md\n\n" +
		"# Run with HTTP API enabled\n" +
		"  symphony run --port 7777\n\n" +
		"# Run with custom logs root\n" +
		"  symphony run --logs-root ./log",
	Args:    cobra.MaximumNArgs(1),
	PreRunE: validateRunInputs,
	RunE:    runService,
}

func init() {
	RunCmd.Flags().IntVar(&runPort, "port", 0, "HTTP observability port (0 disables unless workflow server.port is set)")
	RunCmd.Flags().StringVar(&runLogsRoot, "logs-root", "./log", "Root directory for runtime logs")
}

func validateRunInputs(cmd *cobra.Command, args []string) error {
	if err := subcommands.ValidateWorkflowArgCount(args); err != nil {
		return err
	}
	if _, err := subcommands.ResolveWorkflowPath(args); err != nil {
		return err
	}
	if runPort < 0 {
		return fmt.Errorf("invalid --port; must be >= 0")
	}
	if runLogsRoot == "" {
		return fmt.Errorf("invalid --logs-root; value must be non-empty")
	}
	runPortExplicit = cmd.Flags().Changed("port")

	cmd.SilenceUsage = true
	return nil
}

func runService(_ *cobra.Command, args []string) error {
	workflowPath, err := subcommands.ResolveWorkflowPath(args)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(runLogsRoot, 0o755); err != nil {
		return fmt.Errorf("failed to create logs root; %w", err)
	}

	logger, err := logging.New(runLogsRoot)
	if err != nil {
		return fmt.Errorf("failed to initialize logger; %w", err)
	}

	initOpts := config.InitOptions{WorkflowPath: workflowPath, Logger: logger}
	if err := config.Init(initOpts); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg := config.Get()
	effectivePort := subcommands.ResolvePort(runPort, cfg.Server.Port, runPortExplicit)

	tracker := linear.NewAdapter(cfg.Tracker, logger)
	workspaceManager := workspace.NewManager(cfg.Workspace.Root, cfg.Hooks, logger)
	agentRunner := runner.New(logger, workspaceManager, tracker, tracker)
	service := orchestrator.New(orchestrator.Dependencies{
		Logger:           logger,
		Tracker:          tracker,
		WorkspaceManager: workspaceManager,
		AgentRunner:      agentRunner,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		errCh <- service.Run(ctx)
	}()

	if effectivePort > 0 {
		server := httpserver.New(httpserver.Dependencies{
			Logger:       logger,
			Port:         effectivePort,
			Orchestrator: service,
		})
		go func() {
			errCh <- server.Run(ctx)
		}()
	}

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		if err == nil {
			return nil
		}
		return err
	}
}
