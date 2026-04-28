package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthropics/symphony/internal/codex"
	"github.com/anthropics/symphony/internal/config"
	"github.com/anthropics/symphony/internal/domain"
	"github.com/anthropics/symphony/internal/persona"
	"github.com/anthropics/symphony/internal/workspace"
)

// RunnerDeps holds injected dependencies for the agent runner.
type RunnerDeps struct {
	Config         func() *config.Config
	PromptTemplate func() string
	Workspace      *workspace.Manager
	PersonaReg     *persona.Registry
	PersonaStore   *persona.AssignmentStore
	OnCodexEvent   func(domain.CodexUpdateEvent)
	OnRuntimeInfo  func(domain.WorkerRuntimeInfoEvent)
	GraphQLExec    codex.GraphQLExecutor
}

// GraphQLExecutor is provided by the tracker/linear package for the linear_graphql tool.
// Defined here to avoid import cycles; the actual implementation lives in tracker/linear.

// Run executes a complete agent attempt for one issue. It is designed to be called
// as a goroutine by the orchestrator. SPEC Section 16.5.
func Run(ctx context.Context, issue domain.Issue, attempt *int, deps RunnerDeps) error {
	cfg := deps.Config()
	log := slog.With("issue_id", issue.ID, "issue_identifier", issue.Identifier)

	// Resolve persona for this issue. SPEC Appendix B.5.3.
	var resolvedPersona *persona.Persona
	var personaName string
	if deps.PersonaReg != nil && deps.PersonaStore != nil {
		resolvedPersona = persona.ResolvePersona(
			issue,
			deps.PersonaReg,
			deps.PersonaStore,
			cfg.Personas.LabelPrefix,
		)
		if resolvedPersona != nil {
			personaName = resolvedPersona.Name
			log = log.With("persona", personaName)
		}
	}

	// Compute effective config with persona overrides. SPEC Appendix B.6.
	effectiveCfg := cfg
	if resolvedPersona != nil {
		effectiveCfg = persona.MergeConfig(cfg, resolvedPersona)
	}

	// 1. Create/reuse workspace.
	ws, err := deps.Workspace.CreateForIssue(ctx, issue.Identifier)
	if err != nil {
		return fmt.Errorf("workspace creation failed: %w", err)
	}
	log.Info("workspace ready", "path", ws.Path, "created_now", ws.CreatedNow)

	// Notify orchestrator of runtime info.
	if deps.OnRuntimeInfo != nil {
		deps.OnRuntimeInfo(domain.WorkerRuntimeInfoEvent{
			IssueID:       issue.ID,
			WorkspacePath: ws.Path,
			PersonaName:   personaName,
		})
	}

	// 2. Run before_run hook.
	if err := deps.Workspace.RunBeforeRunHook(ctx, ws.Path); err != nil {
		return fmt.Errorf("before_run hook failed: %w", err)
	}

	// 3. Build tool specs for the session.
	tools := codex.DefaultToolSpecs()
	if resolvedPersona != nil {
		tools = persona.FilterTools(tools, resolvedPersona)
	}

	// 4. Start Codex session.
	session, err := codex.StartSession(ctx, ws.Path, &effectiveCfg.Codex, tools, deps.GraphQLExec)
	if err != nil {
		deps.Workspace.RunAfterRunHook(ctx, ws.Path)
		return fmt.Errorf("codex session startup failed: %w", err)
	}
	defer func() {
		session.Stop()
		deps.Workspace.RunAfterRunHook(ctx, ws.Path)
	}()

	// 5. Turn loop.
	maxTurns := effectiveCfg.Agent.MaxTurns
	promptTemplate := deps.PromptTemplate()

	for turnNumber := 1; turnNumber <= maxTurns; turnNumber++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Build the turn prompt.
		var prompt string
		if turnNumber == 1 {
			// First turn: full rendered prompt.
			var personaPromptStr string
			if resolvedPersona != nil && resolvedPersona.PromptTemplate != "" {
				rendered, err := BuildPrompt(resolvedPersona.PromptTemplate, issue, attempt)
				if err != nil {
					return fmt.Errorf("persona prompt render failed: %w", err)
				}
				personaPromptStr = rendered
			}

			tmpl := promptTemplate
			if tmpl == "" {
				tmpl = DefaultPromptTemplate
			}
			workflowPromptStr, err := BuildPrompt(tmpl, issue, attempt)
			if err != nil {
				return fmt.Errorf("workflow prompt render failed: %w", err)
			}

			prompt = ComposePrompt(personaPromptStr, workflowPromptStr)
		} else {
			// Continuation turn: guidance only.
			prompt = BuildContinuationPrompt(turnNumber, maxTurns)
		}

		log.Info("starting turn", "turn", turnNumber, "max_turns", maxTurns)

		// Run the turn.
		onMsg := func(evt domain.CodexUpdateEvent) {
			evt.TurnCount = turnNumber
			if deps.OnCodexEvent != nil {
				deps.OnCodexEvent(evt)
			}
		}
		if err := session.RunTurn(ctx, prompt, issue, onMsg); err != nil {
			return fmt.Errorf("turn %d failed: %w", turnNumber, err)
		}

		// After each turn, re-check issue state via tracker.
		// The orchestrator will handle this via reconciliation; the runner just
		// checks if the context has been cancelled (which happens when the
		// orchestrator detects a terminal/non-active state transition).
		if ctx.Err() != nil {
			log.Info("context cancelled after turn", "turn", turnNumber)
			return nil // clean exit, not an error
		}

		if turnNumber >= maxTurns {
			log.Info("max turns reached", "turns", turnNumber)
			break
		}
	}

	return nil
}

// IsActiveState checks if a state is in the active states list (case-insensitive).
func IsActiveState(state string, activeStates []string) bool {
	normalized := strings.ToLower(state)
	for _, s := range activeStates {
		if strings.ToLower(s) == normalized {
			return true
		}
	}
	return false
}

// IsTerminalState checks if a state is in the terminal states list (case-insensitive).
func IsTerminalState(state string, terminalStates []string) bool {
	normalized := strings.ToLower(state)
	for _, s := range terminalStates {
		if strings.ToLower(s) == normalized {
			return true
		}
	}
	return false
}
