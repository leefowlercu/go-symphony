package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/leefowlercu/go-symphony/internal/workflow"
	"github.com/spf13/viper"
)

type InitOptions struct {
	WorkflowPath string
	Logger       *slog.Logger
}

var (
	mu        sync.RWMutex
	active    EffectiveConfig
	activeDef workflow.Definition
	logger    = slog.Default()
)

func Init(opts InitOptions) error {
	if opts.Logger != nil {
		logger = opts.Logger
	}
	def, cfg, err := loadEffective(opts.WorkflowPath)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	activeDef = def
	active = cfg
	return nil
}

func Reload() error {
	mu.RLock()
	path := active.WorkflowPath
	mu.RUnlock()
	if path == "" {
		return fmt.Errorf("config not initialized")
	}
	def, cfg, err := loadEffective(path)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	activeDef = def
	active = cfg
	return nil
}

func ReloadIfStale() {
	mu.RLock()
	path := active.WorkflowPath
	last := activeDef.ModTime
	mu.RUnlock()
	if path == "" {
		return
	}
	stat, err := os.Stat(path)
	if err != nil {
		return
	}
	if !stat.ModTime().UTC().After(last) {
		return
	}
	if err := Reload(); err != nil {
		logger.Error("config reload failed; keeping last known good", "error", err)
	}
}

func Get() EffectiveConfig {
	mu.RLock()
	defer mu.RUnlock()
	return active
}

func MustGet() EffectiveConfig {
	cfg := Get()
	if cfg.WorkflowPath == "" {
		panic("config not initialized")
	}
	return cfg
}

func ExpandPath(raw string) string {
	if raw == "" {
		return raw
	}
	resolved := resolveEnvToken(raw)
	if strings.HasPrefix(resolved, "~/") || resolved == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			resolved = filepath.Join(home, strings.TrimPrefix(resolved, "~/"))
		}
	}
	if strings.ContainsRune(resolved, os.PathSeparator) {
		abs, err := filepath.Abs(resolved)
		if err == nil {
			return abs
		}
	}
	return resolved
}

func ValidateDispatchConfig() error {
	cfg := Get()
	if cfg.Tracker.Kind == "" {
		return fmt.Errorf("tracker kind missing")
	}
	if cfg.Tracker.Kind != "linear" {
		return fmt.Errorf("unsupported tracker kind: %s", cfg.Tracker.Kind)
	}
	if strings.TrimSpace(cfg.Tracker.APIKey) == "" {
		return fmt.Errorf("missing tracker api key")
	}
	if strings.TrimSpace(cfg.Tracker.ProjectSlug) == "" {
		return fmt.Errorf("missing tracker project slug")
	}
	if strings.TrimSpace(cfg.Codex.Command) == "" {
		return fmt.Errorf("missing codex command")
	}
	return nil
}

func loadEffective(path string) (workflow.Definition, EffectiveConfig, error) {
	def, err := workflow.Load(path)
	if err != nil {
		return workflow.Definition{}, EffectiveConfig{}, err
	}

	cfg := NewDefaultConfig()
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix("SYMPHONY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	registerDefaults(v, cfg)

	if err := mergeWorkflowConfig(v, def.Config); err != nil {
		return workflow.Definition{}, EffectiveConfig{}, fmt.Errorf("decode workflow config; %w", err)
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return workflow.Definition{}, EffectiveConfig{}, fmt.Errorf("unmarshal config; %w", err)
	}

	normalized := normalize(cfg, def.Config)
	normalized.WorkflowPath = path
	normalized.PromptTemplate = def.PromptTemplate

	if err := validate(normalized); err != nil {
		return workflow.Definition{}, EffectiveConfig{}, err
	}
	return def, normalized, nil
}

func registerDefaults(v *viper.Viper, cfg Config) {
	v.SetDefault("tracker.endpoint", cfg.Tracker.Endpoint)
	v.SetDefault("tracker.active_states", cfg.Tracker.ActiveStates)
	v.SetDefault("tracker.terminal_states", cfg.Tracker.TerminalStates)
	v.SetDefault("polling.interval_ms", cfg.Polling.IntervalMS)
	v.SetDefault("workspace.root", cfg.Workspace.Root)
	v.SetDefault("hooks.timeout_ms", cfg.Hooks.TimeoutMS)
	v.SetDefault("agent.max_concurrent_agents", cfg.Agent.MaxConcurrentAgents)
	v.SetDefault("agent.max_turns", cfg.Agent.MaxTurns)
	v.SetDefault("agent.max_retry_backoff_ms", cfg.Agent.MaxRetryBackoffMS)
	v.SetDefault("agent.max_concurrent_agents_by_state", cfg.Agent.MaxConcurrentAgentsByState)
	v.SetDefault("codex.command", cfg.Codex.Command)
	v.SetDefault("codex.approval_policy", cfg.Codex.ApprovalPolicy)
	v.SetDefault("codex.thread_sandbox", cfg.Codex.ThreadSandbox)
	v.SetDefault("codex.turn_sandbox_policy", cfg.Codex.TurnSandboxPolicy)
	v.SetDefault("codex.turn_timeout_ms", cfg.Codex.TurnTimeoutMS)
	v.SetDefault("codex.read_timeout_ms", cfg.Codex.ReadTimeoutMS)
	v.SetDefault("codex.stall_timeout_ms", cfg.Codex.StallTimeoutMS)
	v.SetDefault("server.port", cfg.Server.Port)
}

func mergeWorkflowConfig(v *viper.Viper, cfg map[string]any) error {
	if cfg == nil {
		return nil
	}
	for k, val := range cfg {
		v.Set(k, val)
	}
	return nil
}

func normalize(cfg Config, raw map[string]any) EffectiveConfig {
	applyRawOverrides(&cfg, raw)
	cfg.Tracker.APIKey = resolveTrackerAPIKey(cfg.Tracker.APIKey)
	cfg.Tracker.Endpoint = strings.TrimSpace(cfg.Tracker.Endpoint)
	cfg.Tracker.Kind = strings.TrimSpace(cfg.Tracker.Kind)
	cfg.Tracker.ProjectSlug = strings.TrimSpace(cfg.Tracker.ProjectSlug)
	cfg.Workspace.Root = ExpandPath(cfg.Workspace.Root)
	cfg.Tracker.ActiveStates = normalizeStates(cfg.Tracker.ActiveStates, defaultActiveStates)
	cfg.Tracker.TerminalStates = normalizeStates(cfg.Tracker.TerminalStates, defaultTerminalStates)
	cfg.Hooks.TimeoutMS = positiveOrDefault(cfg.Hooks.TimeoutMS, defaultHooksTimeoutMS)
	cfg.Polling.IntervalMS = positiveOrDefault(cfg.Polling.IntervalMS, defaultPollingMS)
	cfg.Agent.MaxConcurrentAgents = positiveOrDefault(cfg.Agent.MaxConcurrentAgents, defaultMaxConcurrentAgents)
	cfg.Agent.MaxTurns = positiveOrDefault(cfg.Agent.MaxTurns, defaultMaxTurns)
	cfg.Agent.MaxRetryBackoffMS = positiveOrDefault(cfg.Agent.MaxRetryBackoffMS, defaultMaxRetryBackoffMS)
	cfg.Codex.TurnTimeoutMS = positiveOrDefault(cfg.Codex.TurnTimeoutMS, defaultTurnTimeoutMS)
	cfg.Codex.ReadTimeoutMS = positiveOrDefault(cfg.Codex.ReadTimeoutMS, defaultReadTimeoutMS)
	if cfg.Codex.StallTimeoutMS == 0 {
		cfg.Codex.StallTimeoutMS = defaultStallTimeoutMS
	}
	if strings.TrimSpace(cfg.Codex.Command) == "" {
		cfg.Codex.Command = defaultCodexCommand
	}
	if cfg.Codex.ApprovalPolicy == nil {
		cfg.Codex.ApprovalPolicy = defaultApprovalPolicy
	}
	if strings.TrimSpace(cfg.Codex.ThreadSandbox) == "" {
		cfg.Codex.ThreadSandbox = defaultThreadSandbox
	}
	if cfg.Codex.TurnSandboxPolicy == nil || len(cfg.Codex.TurnSandboxPolicy) == 0 {
		cfg.Codex.TurnSandboxPolicy = map[string]any{"type": "workspaceWrite", "writableRoots": []string{cfg.Workspace.Root}}
	}
	cfg.Agent.MaxConcurrentAgentsByState = normalizePerState(cfg.Agent.MaxConcurrentAgentsByState)

	return EffectiveConfig{Config: cfg}
}

func applyRawOverrides(cfg *Config, raw map[string]any) {
	if cfg == nil || raw == nil {
		return
	}

	if v, ok := getPath(raw, "tracker", "active_states"); ok {
		if list := parseStringList(v); len(list) > 0 {
			cfg.Tracker.ActiveStates = list
		}
	}
	if v, ok := getPath(raw, "tracker", "terminal_states"); ok {
		if list := parseStringList(v); len(list) > 0 {
			cfg.Tracker.TerminalStates = list
		}
	}
	if v, ok := getPath(raw, "polling", "interval_ms"); ok {
		if n, ok := ParseIntLike(v); ok {
			cfg.Polling.IntervalMS = n
		}
	}
	if v, ok := getPath(raw, "hooks", "timeout_ms"); ok {
		if n, ok := ParseIntLike(v); ok {
			cfg.Hooks.TimeoutMS = n
		}
	}
	if v, ok := getPath(raw, "agent", "max_concurrent_agents"); ok {
		if n, ok := ParseIntLike(v); ok {
			cfg.Agent.MaxConcurrentAgents = n
		}
	}
	if v, ok := getPath(raw, "agent", "max_turns"); ok {
		if n, ok := ParseIntLike(v); ok {
			cfg.Agent.MaxTurns = n
		}
	}
	if v, ok := getPath(raw, "agent", "max_retry_backoff_ms"); ok {
		if n, ok := ParseIntLike(v); ok {
			cfg.Agent.MaxRetryBackoffMS = n
		}
	}
	if v, ok := getPath(raw, "codex", "turn_timeout_ms"); ok {
		if n, ok := ParseIntLike(v); ok {
			cfg.Codex.TurnTimeoutMS = n
		}
	}
	if v, ok := getPath(raw, "codex", "read_timeout_ms"); ok {
		if n, ok := ParseIntLike(v); ok {
			cfg.Codex.ReadTimeoutMS = n
		}
	}
	if v, ok := getPath(raw, "codex", "stall_timeout_ms"); ok {
		if n, ok := ParseIntLike(v); ok {
			cfg.Codex.StallTimeoutMS = n
		}
	}
	if v, ok := getPath(raw, "server", "port"); ok {
		if n, ok := ParseIntLike(v); ok {
			cfg.Server.Port = n
		}
	}
	if v, ok := getPath(raw, "agent", "max_concurrent_agents_by_state"); ok {
		if m, ok := v.(map[string]any); ok {
			out := map[string]int{}
			for key, val := range m {
				if n, ok := ParseIntLike(val); ok && n > 0 {
					out[key] = n
				}
			}
			if len(out) > 0 {
				cfg.Agent.MaxConcurrentAgentsByState = out
			}
		}
	}
}

func validate(cfg EffectiveConfig) error {
	if strings.TrimSpace(cfg.Workspace.Root) == "" {
		return fmt.Errorf("invalid workspace.root")
	}
	return nil
}

func resolveTrackerAPIKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return strings.TrimSpace(os.Getenv("LINEAR_API_KEY"))
	}
	if strings.HasPrefix(trimmed, "$") {
		return strings.TrimSpace(os.Getenv(strings.TrimPrefix(trimmed, "$")))
	}
	return trimmed
}

func resolveEnvToken(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "$") {
		return os.Getenv(strings.TrimPrefix(trimmed, "$"))
	}
	return value
}

func normalizeStates(values []string, fallback []string) []string {
	if len(values) == 0 {
		return append([]string(nil), fallback...)
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			s := strings.TrimSpace(part)
			if s == "" {
				continue
			}
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), fallback...)
	}
	return out
}

func positiveOrDefault(v int, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func normalizePerState(raw map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range raw {
		if v <= 0 {
			continue
		}
		state := strings.ToLower(strings.TrimSpace(k))
		if state == "" {
			continue
		}
		out[state] = v
	}
	return out
}

func getPath(root map[string]any, keys ...string) (any, bool) {
	var curr any = root
	for _, key := range keys {
		m, ok := curr.(map[string]any)
		if !ok {
			return nil, false
		}
		curr, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return curr, true
}

func parseStringList(v any) []string {
	switch t := v.(type) {
	case string:
		parts := strings.Split(t, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			s := strings.TrimSpace(p)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(t))
		for _, v := range t {
			s, ok := v.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(t))
		for _, s := range t {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func ParseIntLike(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}
