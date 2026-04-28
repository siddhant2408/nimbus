package persona

import (
	"github.com/anthropics/symphony/internal/codex"
	"github.com/anthropics/symphony/internal/config"
)

// Persona represents a named agent identity. SPEC Appendix B.4.1.
type Persona struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	SourcePath     string          `json:"source_path"`
	Overrides      PersonaOverrides `json:"overrides"`
	PromptTemplate string          `json:"prompt_template,omitempty"`
}

// PersonaOverrides are config values that override the base workflow config. SPEC B.2.1.
type PersonaOverrides struct {
	Agent *AgentOverrides `json:"agent,omitempty" yaml:"agent,omitempty"`
	Codex *CodexOverrides `json:"codex,omitempty" yaml:"codex,omitempty"`
	Tools *ToolOverrides  `json:"tools,omitempty" yaml:"tools,omitempty"`
}

// AgentOverrides contains overridable agent config fields.
type AgentOverrides struct {
	MaxTurns *int `json:"max_turns,omitempty" yaml:"max_turns,omitempty"`
}

// CodexOverrides contains overridable codex config fields.
type CodexOverrides struct {
	ApprovalPolicy *string `json:"approval_policy,omitempty" yaml:"approval_policy,omitempty"`
	Model          *string `json:"model,omitempty" yaml:"model,omitempty"`
	TurnTimeoutMs  *int    `json:"turn_timeout_ms,omitempty" yaml:"turn_timeout_ms,omitempty"`
}

// ToolOverrides controls tool visibility per persona.
type ToolOverrides struct {
	Allow []string `json:"allow,omitempty" yaml:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty" yaml:"deny,omitempty"`
}

// MergeConfig produces an effective config by overlaying persona overrides. SPEC B.6.
func MergeConfig(base *config.Config, p *Persona) *config.Config {
	if p == nil {
		return base
	}

	// Deep copy the base config.
	effective := *base
	effective.Agent = base.Agent
	effective.Codex = base.Codex

	// Copy the state map to avoid mutating the original.
	effective.Agent.MaxConcurrentAgentsByState = make(map[string]int, len(base.Agent.MaxConcurrentAgentsByState))
	for k, v := range base.Agent.MaxConcurrentAgentsByState {
		effective.Agent.MaxConcurrentAgentsByState[k] = v
	}

	if p.Overrides.Agent != nil {
		if p.Overrides.Agent.MaxTurns != nil {
			effective.Agent.MaxTurns = *p.Overrides.Agent.MaxTurns
		}
	}

	if p.Overrides.Codex != nil {
		if p.Overrides.Codex.ApprovalPolicy != nil {
			effective.Codex.ApprovalPolicy = *p.Overrides.Codex.ApprovalPolicy
		}
		if p.Overrides.Codex.Model != nil {
			effective.Codex.Model = *p.Overrides.Codex.Model
		}
		if p.Overrides.Codex.TurnTimeoutMs != nil {
			effective.Codex.TurnTimeoutMs = *p.Overrides.Codex.TurnTimeoutMs
		}
	}

	return &effective
}

// FilterTools applies persona tool allow/deny lists to the tool set. SPEC B.12.5.
func FilterTools(tools []codex.ToolSpec, p *Persona) []codex.ToolSpec {
	if p == nil || p.Overrides.Tools == nil {
		return tools
	}

	if len(p.Overrides.Tools.Allow) > 0 {
		allowed := make(map[string]struct{}, len(p.Overrides.Tools.Allow))
		for _, name := range p.Overrides.Tools.Allow {
			allowed[name] = struct{}{}
		}
		var result []codex.ToolSpec
		for _, t := range tools {
			if _, ok := allowed[t.Name]; ok {
				result = append(result, t)
			}
		}
		return result
	}

	if len(p.Overrides.Tools.Deny) > 0 {
		denied := make(map[string]struct{}, len(p.Overrides.Tools.Deny))
		for _, name := range p.Overrides.Tools.Deny {
			denied[name] = struct{}{}
		}
		var result []codex.ToolSpec
		for _, t := range tools {
			if _, ok := denied[t.Name]; !ok {
				result = append(result, t)
			}
		}
		return result
	}

	return tools
}
