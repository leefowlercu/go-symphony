package config

type Config struct {
	Tracker   TrackerConfig   `yaml:"tracker" mapstructure:"tracker"`
	Polling   PollingConfig   `yaml:"polling" mapstructure:"polling"`
	Workspace WorkspaceConfig `yaml:"workspace" mapstructure:"workspace"`
	Hooks     HooksConfig     `yaml:"hooks" mapstructure:"hooks"`
	Agent     AgentConfig     `yaml:"agent" mapstructure:"agent"`
	Codex     CodexConfig     `yaml:"codex" mapstructure:"codex"`
	Server    ServerConfig    `yaml:"server" mapstructure:"server"`
}

type TrackerConfig struct {
	Kind           string   `yaml:"kind" mapstructure:"kind"`
	Endpoint       string   `yaml:"endpoint" mapstructure:"endpoint"`
	APIKey         string   `yaml:"api_key" mapstructure:"api_key"`
	ProjectSlug    string   `yaml:"project_slug" mapstructure:"project_slug"`
	ActiveStates   []string `yaml:"active_states" mapstructure:"active_states"`
	TerminalStates []string `yaml:"terminal_states" mapstructure:"terminal_states"`
}

type PollingConfig struct {
	IntervalMS int `yaml:"interval_ms" mapstructure:"interval_ms"`
}

type WorkspaceConfig struct {
	Root string `yaml:"root" mapstructure:"root"`
}

type HooksConfig struct {
	AfterCreate  *string `yaml:"after_create" mapstructure:"after_create"`
	BeforeRun    *string `yaml:"before_run" mapstructure:"before_run"`
	AfterRun     *string `yaml:"after_run" mapstructure:"after_run"`
	BeforeRemove *string `yaml:"before_remove" mapstructure:"before_remove"`
	TimeoutMS    int     `yaml:"timeout_ms" mapstructure:"timeout_ms"`
}

type AgentConfig struct {
	MaxConcurrentAgents        int            `yaml:"max_concurrent_agents" mapstructure:"max_concurrent_agents"`
	MaxTurns                   int            `yaml:"max_turns" mapstructure:"max_turns"`
	MaxRetryBackoffMS          int            `yaml:"max_retry_backoff_ms" mapstructure:"max_retry_backoff_ms"`
	MaxConcurrentAgentsByState map[string]int `yaml:"max_concurrent_agents_by_state" mapstructure:"max_concurrent_agents_by_state"`
}

type CodexConfig struct {
	Command           string         `yaml:"command" mapstructure:"command"`
	ApprovalPolicy    any            `yaml:"approval_policy" mapstructure:"approval_policy"`
	ThreadSandbox     string         `yaml:"thread_sandbox" mapstructure:"thread_sandbox"`
	TurnSandboxPolicy map[string]any `yaml:"turn_sandbox_policy" mapstructure:"turn_sandbox_policy"`
	TurnTimeoutMS     int            `yaml:"turn_timeout_ms" mapstructure:"turn_timeout_ms"`
	ReadTimeoutMS     int            `yaml:"read_timeout_ms" mapstructure:"read_timeout_ms"`
	StallTimeoutMS    int            `yaml:"stall_timeout_ms" mapstructure:"stall_timeout_ms"`
}

type ServerConfig struct {
	Port int `yaml:"port" mapstructure:"port"`
}

type EffectiveConfig struct {
	Config
	WorkflowPath   string
	PromptTemplate string
}
