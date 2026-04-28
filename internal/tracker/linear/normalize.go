package linear

import (
	"strings"
	"time"

	"github.com/siddhant2408/nimbus/internal/domain"
)

// normalizeIssueNodes converts a slice of raw GraphQL issue nodes into
// domain.Issue values, applying the optional assignee filter.
func normalizeIssueNodes(nodes []any, filter *assigneeMatch) []domain.Issue {
	issues := make([]domain.Issue, 0, len(nodes))
	for _, n := range nodes {
		m, ok := n.(map[string]any)
		if !ok {
			continue
		}
		issue, ok := normalizeIssue(m, filter)
		if !ok {
			continue
		}
		issues = append(issues, issue)
	}
	return issues
}

// normalizeIssue converts a single GraphQL issue map into a domain.Issue.
// It returns false if the issue should be excluded (e.g. assignee mismatch).
func normalizeIssue(m map[string]any, filter *assigneeMatch) (domain.Issue, bool) {
	assigneeID := extractAssigneeID(m)

	if !matchesAssignee(assigneeID, filter) {
		return domain.Issue{}, false
	}

	issue := domain.Issue{
		ID:          getString(m, "id"),
		Identifier:  getString(m, "identifier"),
		Title:       getString(m, "title"),
		Description: getStringPtr(m, "description"),
		Priority:    parsePriority(m["priority"]),
		State:       extractStateName(m),
		BranchName:  getStringPtr(m, "branchName"),
		URL:         getStringPtr(m, "url"),
		AssigneeID:  assigneeID,
		Labels:      extractLabels(m),
		BlockedBy:   extractBlockers(m),
		CreatedAt:   parseISO8601(m["createdAt"]),
		UpdatedAt:   parseISO8601(m["updatedAt"]),
	}

	return issue, true
}

// ---- Field extraction helpers -----------------------------------------------

// extractStateName returns state.name from a GraphQL issue node.
func extractStateName(m map[string]any) string {
	stateObj, _ := m["state"].(map[string]any)
	if stateObj == nil {
		return ""
	}
	name, _ := stateObj["name"].(string)
	return name
}

// extractAssigneeID returns a pointer to assignee.id, or nil when absent.
func extractAssigneeID(m map[string]any) *string {
	assignee, _ := m["assignee"].(map[string]any)
	if assignee == nil {
		return nil
	}
	id, ok := assignee["id"].(string)
	if !ok || id == "" {
		return nil
	}
	return &id
}

// extractLabels returns lowercased label names from labels.nodes[].name.
func extractLabels(m map[string]any) []string {
	labelsObj, _ := m["labels"].(map[string]any)
	if labelsObj == nil {
		return nil
	}
	nodes, _ := labelsObj["nodes"].([]any)
	if len(nodes) == 0 {
		return nil
	}

	labels := make([]string, 0, len(nodes))
	for _, n := range nodes {
		node, ok := n.(map[string]any)
		if !ok {
			continue
		}
		name, ok := node["name"].(string)
		if !ok {
			continue
		}
		labels = append(labels, strings.ToLower(name))
	}
	return labels
}

// extractBlockers returns BlockerRef values from inverseRelations.nodes where
// the relation type is "blocks" (case-insensitive comparison).
func extractBlockers(m map[string]any) []domain.BlockerRef {
	relObj, _ := m["inverseRelations"].(map[string]any)
	if relObj == nil {
		return nil
	}
	nodes, _ := relObj["nodes"].([]any)
	if len(nodes) == 0 {
		return nil
	}

	var blockers []domain.BlockerRef
	for _, n := range nodes {
		rel, ok := n.(map[string]any)
		if !ok {
			continue
		}
		relType, _ := rel["type"].(string)
		if !strings.EqualFold(strings.TrimSpace(relType), "blocks") {
			continue
		}
		blockerIssue, _ := rel["issue"].(map[string]any)
		if blockerIssue == nil {
			continue
		}

		ref := domain.BlockerRef{
			ID:         getStringPtr(blockerIssue, "id"),
			Identifier: getStringPtr(blockerIssue, "identifier"),
			State:      extractStateNamePtr(blockerIssue),
		}
		blockers = append(blockers, ref)
	}
	return blockers
}

// extractStateNamePtr returns a pointer to state.name, or nil.
func extractStateNamePtr(m map[string]any) *string {
	stateObj, _ := m["state"].(map[string]any)
	if stateObj == nil {
		return nil
	}
	name, ok := stateObj["name"].(string)
	if !ok {
		return nil
	}
	return &name
}

// ---- Parsing helpers --------------------------------------------------------

// parsePriority returns a pointer to the integer priority, or nil for
// non-integer values.
func parsePriority(v any) *int {
	switch p := v.(type) {
	case float64:
		// JSON numbers decode as float64; only accept integer values.
		i := int(p)
		if float64(i) != p {
			return nil
		}
		return &i
	case int:
		return &p
	case int64:
		i := int(p)
		return &i
	default:
		return nil
	}
}

// parseISO8601 parses an ISO-8601 datetime string into a *time.Time.
// Returns nil for nil/empty/unparseable values.
func parseISO8601(v any) *time.Time {
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}

	// Try RFC 3339 first (most common from Linear).
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}
	// Try RFC 3339 with nanoseconds.
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return &t
	}
	// Fallback: ISO-8601 without timezone (assume UTC).
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		utc := t.UTC()
		return &utc
	}

	return nil
}

// ---- Generic map helpers ----------------------------------------------------

func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func getStringPtr(m map[string]any, key string) *string {
	v, ok := m[key].(string)
	if !ok {
		return nil
	}
	return &v
}
