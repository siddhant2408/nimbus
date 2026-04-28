package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkflowDefinition is the parsed WORKFLOW.md payload. SPEC Section 4.1.2.
type WorkflowDefinition struct {
	Config         *Config
	PromptTemplate string
}

// LoadWorkflow reads and parses a WORKFLOW.md file.
func LoadWorkflow(path string) (*WorkflowDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("missing_workflow_file: %w", err)
	}

	rawConfig, promptTemplate, err := splitFrontMatter(string(data))
	if err != nil {
		return nil, err
	}

	cfg, err := parseConfig(rawConfig, filepath.Dir(path))
	if err != nil {
		return nil, err
	}

	return &WorkflowDefinition{
		Config:         cfg,
		PromptTemplate: strings.TrimSpace(promptTemplate),
	}, nil
}

// splitFrontMatter splits YAML front matter from the prompt body.
func splitFrontMatter(content string) (map[string]any, string, error) {
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		// No front matter; entire file is the prompt body.
		return map[string]any{}, content, nil
	}

	// Find the opening and closing --- markers.
	trimmed := strings.TrimSpace(content)
	rest := trimmed[3:] // skip first ---

	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		// Only one ---, treat entire content as prompt.
		return map[string]any{}, content, nil
	}

	yamlPart := rest[:idx]
	promptPart := rest[idx+4:] // skip \n---

	var raw any
	if err := yaml.Unmarshal([]byte(yamlPart), &raw); err != nil {
		return nil, "", fmt.Errorf("workflow_parse_error: %w", err)
	}

	if raw == nil {
		return map[string]any{}, strings.TrimSpace(promptPart), nil
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("workflow_front_matter_not_a_map")
	}

	return m, promptPart, nil
}

// parseConfig applies raw YAML values onto a default Config struct.
func parseConfig(raw map[string]any, workflowDir string) (*Config, error) {
	cfg := DefaultConfig()

	if tracker, ok := raw["tracker"].(map[string]any); ok {
		if v, ok := tracker["kind"].(string); ok {
			cfg.Tracker.Kind = v
		}
		if v, ok := tracker["endpoint"].(string); ok {
			cfg.Tracker.Endpoint = v
		}
		if v, ok := tracker["api_key"].(string); ok {
			cfg.Tracker.APIKey = resolveEnvVar(v)
		}
		if v, ok := tracker["project_slug"].(string); ok {
			cfg.Tracker.ProjectSlug = v
		}
		if v, ok := tracker["assignee"].(string); ok {
			cfg.Tracker.Assignee = v
		}
		if v, ok := tracker["active_states"].([]any); ok {
			cfg.Tracker.ActiveStates = toStringSlice(v)
		}
		if v, ok := tracker["terminal_states"].([]any); ok {
			cfg.Tracker.TerminalStates = toStringSlice(v)
		}
	}

	if polling, ok := raw["polling"].(map[string]any); ok {
		if v, ok := toInt(polling["interval_ms"]); ok {
			cfg.Polling.IntervalMs = v
		}
	}

	if workspace, ok := raw["workspace"].(map[string]any); ok {
		if v, ok := workspace["root"].(string); ok {
			cfg.Workspace.Root = resolvePath(v, workflowDir)
		}
	}

	if agent, ok := raw["agent"].(map[string]any); ok {
		if v, ok := toInt(agent["max_concurrent_agents"]); ok {
			cfg.Agent.MaxConcurrentAgents = v
		}
		if v, ok := toInt(agent["max_turns"]); ok {
			cfg.Agent.MaxTurns = v
		}
		if v, ok := toInt(agent["max_retry_backoff_ms"]); ok {
			cfg.Agent.MaxRetryBackoffMs = v
		}
		if m, ok := agent["max_concurrent_agents_by_state"].(map[string]any); ok {
			byState := make(map[string]int, len(m))
			for k, val := range m {
				if iv, ok := toInt(val); ok && iv > 0 {
					byState[strings.ToLower(k)] = iv
				}
			}
			cfg.Agent.MaxConcurrentAgentsByState = byState
		}
	}

	if codex, ok := raw["codex"].(map[string]any); ok {
		if v, ok := codex["command"].(string); ok {
			cfg.Codex.Command = v
		}
		if v := codex["approval_policy"]; v != nil {
			cfg.Codex.ApprovalPolicy = v
		}
		if v, ok := codex["thread_sandbox"].(string); ok {
			cfg.Codex.ThreadSandbox = v
		}
		if v, ok := codex["turn_sandbox_policy"].(map[string]any); ok {
			cfg.Codex.TurnSandboxPolicy = v
		}
		if v, ok := toInt(codex["turn_timeout_ms"]); ok {
			cfg.Codex.TurnTimeoutMs = v
		}
		if v, ok := toInt(codex["read_timeout_ms"]); ok {
			cfg.Codex.ReadTimeoutMs = v
		}
		if v, ok := toInt(codex["stall_timeout_ms"]); ok {
			cfg.Codex.StallTimeoutMs = v
		}
		if v, ok := codex["model"].(string); ok {
			cfg.Codex.Model = v
		}
	}

	if hooks, ok := raw["hooks"].(map[string]any); ok {
		if v, ok := hooks["after_create"].(string); ok {
			cfg.Hooks.AfterCreate = v
		}
		if v, ok := hooks["before_run"].(string); ok {
			cfg.Hooks.BeforeRun = v
		}
		if v, ok := hooks["after_run"].(string); ok {
			cfg.Hooks.AfterRun = v
		}
		if v, ok := hooks["before_remove"].(string); ok {
			cfg.Hooks.BeforeRemove = v
		}
		if v, ok := toInt(hooks["timeout_ms"]); ok {
			cfg.Hooks.TimeoutMs = v
		}
	}

	if server, ok := raw["server"].(map[string]any); ok {
		if v, ok := toInt(server["port"]); ok {
			cfg.Server.Port = v
		}
		if v, ok := server["host"].(string); ok {
			cfg.Server.Host = v
		}
	}

	if personas, ok := raw["personas"].(map[string]any); ok {
		if v, ok := personas["directory"].(string); ok {
			cfg.Personas.Directory = resolvePath(v, workflowDir)
		}
		if v, ok := personas["label_prefix"].(string); ok {
			cfg.Personas.LabelPrefix = v
		}
	}

	// If workspace root is still empty, use a temp directory default.
	if cfg.Workspace.Root == "" {
		cfg.Workspace.Root = filepath.Join(os.TempDir(), "symphony_workspaces")
	}

	// If personas directory is relative and wasn't resolved, resolve it now.
	if cfg.Personas.Directory != "" && !filepath.IsAbs(cfg.Personas.Directory) {
		cfg.Personas.Directory = filepath.Join(workflowDir, cfg.Personas.Directory)
	}

	return cfg, nil
}

// resolveEnvVar resolves $VAR_NAME references to environment values.
func resolveEnvVar(val string) string {
	if strings.HasPrefix(val, "$") {
		envName := val[1:]
		return os.Getenv(envName)
	}
	return val
}

// resolvePath expands ~ and $VAR references and resolves relative paths.
func resolvePath(val string, baseDir string) string {
	// Expand $VAR references.
	val = resolveEnvVar(val)

	// Expand ~ to home directory.
	if strings.HasPrefix(val, "~/") || val == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			val = filepath.Join(home, val[1:])
		}
	}

	// Resolve relative paths against baseDir.
	if !filepath.IsAbs(val) {
		val = filepath.Join(baseDir, val)
	}

	return filepath.Clean(val)
}

// toStringSlice converts []any to []string, skipping non-string values.
func toStringSlice(in []any) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// toInt attempts to convert a YAML-decoded value to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
