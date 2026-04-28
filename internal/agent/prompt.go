package agent

import (
	"fmt"
	"strings"

	"github.com/siddhant2408/nimbus/internal/domain"
	"github.com/osteele/liquid"
)

// BuildPrompt renders the workflow prompt template with issue and attempt variables.
// Uses Liquid-compatible strict rendering. SPEC Section 12.
func BuildPrompt(promptTemplate string, issue domain.Issue, attempt *int) (string, error) {
	engine := liquid.NewEngine()

	bindings := map[string]any{
		"issue":   issueToMap(issue),
		"attempt": attempt,
	}

	out, err := engine.ParseAndRenderString(promptTemplate, bindings)
	if err != nil {
		return "", fmt.Errorf("template_render_error: %w", err)
	}

	return out, nil
}

// ComposePrompt composes persona prompt + workflow prompt with a separator.
// SPEC Appendix B.7.
func ComposePrompt(personaPrompt, workflowPrompt string) string {
	personaPrompt = strings.TrimSpace(personaPrompt)
	workflowPrompt = strings.TrimSpace(workflowPrompt)

	if personaPrompt == "" {
		return workflowPrompt
	}
	if workflowPrompt == "" {
		return personaPrompt
	}
	return personaPrompt + "\n\n---\n\n" + workflowPrompt
}

// BuildContinuationPrompt returns guidance for continuation turns (turn 2+).
// The full task prompt is already in the thread history.
func BuildContinuationPrompt(turnNumber, maxTurns int) string {
	return fmt.Sprintf(
		"Continue working on the issue. This is turn %d of %d. "+
			"Review your progress so far in the conversation history and continue from where you left off. "+
			"If the work is complete, ensure all changes are committed and the issue state is updated.",
		turnNumber, maxTurns,
	)
}

// DefaultPromptTemplate is the fallback when the workflow body is empty.
const DefaultPromptTemplate = `You are working on a Linear issue.

Identifier: {{ issue.identifier }}
Title: {{ issue.title }}

Body:
{% if issue.description %}
{{ issue.description }}
{% else %}
No description provided.
{% endif %}`

// issueToMap converts an Issue struct to a map[string]any for template rendering.
func issueToMap(issue domain.Issue) map[string]any {
	m := map[string]any{
		"id":          issue.ID,
		"identifier":  issue.Identifier,
		"title":       issue.Title,
		"state":       issue.State,
		"labels":      issue.Labels,
		"description": "",
		"priority":    nil,
		"branch_name": "",
		"url":         "",
		"assignee_id": "",
		"created_at":  "",
		"updated_at":  "",
	}

	if issue.Description != nil {
		m["description"] = *issue.Description
	}
	if issue.Priority != nil {
		m["priority"] = *issue.Priority
	}
	if issue.BranchName != nil {
		m["branch_name"] = *issue.BranchName
	}
	if issue.URL != nil {
		m["url"] = *issue.URL
	}
	if issue.AssigneeID != nil {
		m["assignee_id"] = *issue.AssigneeID
	}
	if issue.CreatedAt != nil {
		m["created_at"] = issue.CreatedAt.Format("2006-01-02T15:04:05Z")
	}
	if issue.UpdatedAt != nil {
		m["updated_at"] = issue.UpdatedAt.Format("2006-01-02T15:04:05Z")
	}

	// Convert blockers to template-friendly maps.
	blockers := make([]map[string]any, 0, len(issue.BlockedBy))
	for _, b := range issue.BlockedBy {
		bm := map[string]any{"id": "", "identifier": "", "state": ""}
		if b.ID != nil {
			bm["id"] = *b.ID
		}
		if b.Identifier != nil {
			bm["identifier"] = *b.Identifier
		}
		if b.State != nil {
			bm["state"] = *b.State
		}
		blockers = append(blockers, bm)
	}
	m["blocked_by"] = blockers

	return m
}
