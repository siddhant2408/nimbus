package config

// Config holds the typed runtime configuration derived from WORKFLOW.md front matter.
// Maps to SPEC Section 6.4.
type Config struct {
	Tracker   TrackerConfig
	Polling   PollingConfig
	Workspace WorkspaceConfig
	Agent     AgentConfig
	Codex     CodexConfig
	Hooks     HooksConfig
	Server    ServerConfig
	Personas  PersonasConfig
}

// TrackerConfig holds issue tracker settings. SPEC Section 5.3.1.
type TrackerConfig struct {
	Kind           string   `yaml:"kind"`
	Endpoint       string   `yaml:"endpoint"`
	APIKey         string   `yaml:"api_key"`
	ProjectSlug    string   `yaml:"project_slug"`
	Assignee       string   `yaml:"assignee"`
	ActiveStates   []string `yaml:"active_states"`
	TerminalStates []string `yaml:"terminal_states"`
}

// PollingConfig holds poll cadence settings. SPEC Section 5.3.2.
type PollingConfig struct {
	IntervalMs int `yaml:"interval_ms"`
}

// WorkspaceConfig holds workspace root settings. SPEC Section 5.3.3.
type WorkspaceConfig struct {
	Root string `yaml:"root"`
}

// AgentConfig holds agent concurrency and turn settings. SPEC Section 5.3.5.
type AgentConfig struct {
	MaxConcurrentAgents        int            `yaml:"max_concurrent_agents"`
	MaxTurns                   int            `yaml:"max_turns"`
	MaxRetryBackoffMs          int            `yaml:"max_retry_backoff_ms"`
	MaxConcurrentAgentsByState map[string]int `yaml:"max_concurrent_agents_by_state"`
}

// CodexConfig holds coding-agent subprocess settings. SPEC Section 5.3.6.
type CodexConfig struct {
	Command           string         `yaml:"command"`
	ApprovalPolicy    any            `yaml:"approval_policy"`
	ThreadSandbox     string         `yaml:"thread_sandbox"`
	TurnSandboxPolicy map[string]any `yaml:"turn_sandbox_policy"`
	TurnTimeoutMs     int            `yaml:"turn_timeout_ms"`
	ReadTimeoutMs     int            `yaml:"read_timeout_ms"`
	StallTimeoutMs    int            `yaml:"stall_timeout_ms"`
	Model             string         `yaml:"model"`
}

// HooksConfig holds workspace lifecycle hook scripts. SPEC Section 5.3.4.
type HooksConfig struct {
	AfterCreate  string `yaml:"after_create"`
	BeforeRun    string `yaml:"before_run"`
	AfterRun     string `yaml:"after_run"`
	BeforeRemove string `yaml:"before_remove"`
	TimeoutMs    int    `yaml:"timeout_ms"`
}

// ServerConfig holds optional HTTP server settings. SPEC Section 13.7.
type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

// PersonasConfig holds persona extension settings. SPEC Appendix B.3.1.
type PersonasConfig struct {
	Directory   string `yaml:"directory"`
	LabelPrefix string `yaml:"label_prefix"`
}

// DefaultConfig returns a Config with all spec-mandated defaults applied.
func DefaultConfig() *Config {
	return &Config{
		Tracker: TrackerConfig{
			Kind:           "",
			Endpoint:       "https://api.linear.app/graphql",
			ActiveStates:   []string{"Todo", "In Progress"},
			TerminalStates: []string{"Closed", "Cancelled", "Canceled", "Duplicate", "Done"},
		},
		Polling: PollingConfig{
			IntervalMs: 30000,
		},
		Workspace: WorkspaceConfig{},
		Agent: AgentConfig{
			MaxConcurrentAgents:        10,
			MaxTurns:                   20,
			MaxRetryBackoffMs:          300000, // 5 minutes
			MaxConcurrentAgentsByState: map[string]int{},
		},
		Codex: CodexConfig{
			Command:       "codex app-server",
			ThreadSandbox: "workspace-write",
			TurnTimeoutMs: 3600000, // 1 hour
			ReadTimeoutMs: 5000,
			StallTimeoutMs: 300000, // 5 minutes
		},
		Hooks: HooksConfig{
			TimeoutMs: 60000,
		},
		Server: ServerConfig{
			Host: "127.0.0.1",
		},
		Personas: PersonasConfig{
			Directory:   "personas",
			LabelPrefix: "persona",
		},
	}
}
