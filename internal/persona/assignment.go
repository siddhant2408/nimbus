package persona

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/symphony/internal/domain"
)

// Assignment represents a persona-to-issue mapping. SPEC B.4.2.
type Assignment struct {
	PersonaName string    `json:"persona_name"`
	AssignedAt  time.Time `json:"assigned_at"`
	Source      string    `json:"source"` // "label" or "persisted"
}

// AssignmentStore persists persona assignments locally. SPEC B.5.2.
type AssignmentStore struct {
	mu          sync.RWMutex
	path        string
	assignments map[string]Assignment // issue_id -> assignment
}

// NewAssignmentStore creates or loads a persona assignment store.
func NewAssignmentStore(workspaceRoot string) *AssignmentStore {
	dir := filepath.Join(workspaceRoot, ".symphony")
	path := filepath.Join(dir, "persona_assignments.json")

	store := &AssignmentStore{
		path:        path,
		assignments: make(map[string]Assignment),
	}

	// Load existing assignments.
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &store.assignments); err != nil {
			slog.Warn("failed to parse persona assignments file", "path", path, "error", err)
		}
	}

	return store
}

// Get returns the persisted assignment for an issue, if any.
func (s *AssignmentStore) Get(issueID string) (Assignment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.assignments[issueID]
	return a, ok
}

// Set persists a persona assignment.
func (s *AssignmentStore) Set(issueID string, a Assignment) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.assignments[issueID] = a
	s.persist()
}

// Remove deletes a persisted assignment.
func (s *AssignmentStore) Remove(issueID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.assignments, issueID)
	s.persist()
}

// All returns all persisted assignments.
func (s *AssignmentStore) All() map[string]Assignment {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]Assignment, len(s.assignments))
	for k, v := range s.assignments {
		result[k] = v
	}
	return result
}

func (s *AssignmentStore) persist() {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("failed to create persona assignments directory", "error", err)
		return
	}

	data, err := json.MarshalIndent(s.assignments, "", "  ")
	if err != nil {
		slog.Error("failed to marshal persona assignments", "error", err)
		return
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		slog.Error("failed to write persona assignments", "path", s.path, "error", err)
	}
}

// ResolvePersona resolves the persona for an issue using label-based assignment
// with local persistence fallback. SPEC B.5.3 / B.12.1.
func ResolvePersona(
	issue domain.Issue,
	registry *Registry,
	store *AssignmentStore,
	labelPrefix string,
) *Persona {
	if registry == nil {
		return nil
	}

	prefix := labelPrefix + ":"

	// Step 1: Check labels for persona assignment.
	var matchingNames []string
	for _, label := range issue.Labels {
		if strings.HasPrefix(label, prefix) {
			name := label[len(prefix):]
			matchingNames = append(matchingNames, name)
		}
	}

	if len(matchingNames) > 1 {
		slog.Warn("multiple persona labels on issue",
			"issue_identifier", issue.Identifier,
			"labels", matchingNames,
		)
	}

	if len(matchingNames) >= 1 {
		name := matchingNames[0]
		p := registry.Get(name)
		if p != nil {
			// Persist the assignment.
			if store != nil {
				store.Set(issue.ID, Assignment{
					PersonaName: name,
					AssignedAt:  time.Now().UTC(),
					Source:      "label",
				})
			}
			return p
		}
		slog.Warn("unknown persona in label",
			"persona_name", name,
			"issue_identifier", issue.Identifier,
		)
		return nil
	}

	// Step 2: Check local persistence.
	if store != nil {
		if a, ok := store.Get(issue.ID); ok {
			p := registry.Get(a.PersonaName)
			if p != nil {
				return p
			}
		}
	}

	// Step 3: No persona.
	return nil
}
