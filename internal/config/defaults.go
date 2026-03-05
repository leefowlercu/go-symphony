package config

import "os"

const (
	defaultLinearEndpoint      = "https://api.linear.app/graphql"
	defaultPollingMS           = 30_000
	defaultHooksTimeoutMS      = 60_000
	defaultMaxConcurrentAgents = 10
	defaultMaxTurns            = 20
	defaultMaxRetryBackoffMS   = 300_000
	defaultCodexCommand        = "codex app-server"
	defaultTurnTimeoutMS       = 3_600_000
	defaultReadTimeoutMS       = 5_000
	defaultStallTimeoutMS      = 300_000
)

var (
	defaultActiveStates   = []string{"Todo", "In Progress"}
	defaultTerminalStates = []string{"Closed", "Cancelled", "Canceled", "Duplicate", "Done"}
	defaultApprovalPolicy = map[string]any{
		"reject": map[string]any{
			"sandbox_approval": true,
			"rules":            true,
			"mcp_elicitations": true,
		},
	}
	defaultThreadSandbox = "workspace-write"
)

func NewDefaultConfig() Config {
	return Config{
		Tracker: TrackerConfig{
			Endpoint:       defaultLinearEndpoint,
			ActiveStates:   append([]string(nil), defaultActiveStates...),
			TerminalStates: append([]string(nil), defaultTerminalStates...),
		},
		Polling:   PollingConfig{IntervalMS: defaultPollingMS},
		Workspace: WorkspaceConfig{Root: os.TempDir() + "/symphony_workspaces"},
		Hooks:     HooksConfig{TimeoutMS: defaultHooksTimeoutMS},
		Agent: AgentConfig{
			MaxConcurrentAgents:        defaultMaxConcurrentAgents,
			MaxTurns:                   defaultMaxTurns,
			MaxRetryBackoffMS:          defaultMaxRetryBackoffMS,
			MaxConcurrentAgentsByState: map[string]int{},
		},
		Codex: CodexConfig{
			Command:           defaultCodexCommand,
			ApprovalPolicy:    defaultApprovalPolicy,
			ThreadSandbox:     defaultThreadSandbox,
			TurnSandboxPolicy: map[string]any{"type": "workspaceWrite", "writableRoots": []string{}},
			TurnTimeoutMS:     defaultTurnTimeoutMS,
			ReadTimeoutMS:     defaultReadTimeoutMS,
			StallTimeoutMS:    defaultStallTimeoutMS,
		},
		Server: ServerConfig{Port: 0},
	}
}
