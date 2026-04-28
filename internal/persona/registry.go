package persona

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var validNameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// Registry manages persona definitions loaded from markdown files. SPEC B.3.
type Registry struct {
	mu       sync.RWMutex
	personas map[string]*Persona
	dir      string
}

// NewRegistry creates an empty persona registry.
func NewRegistry() *Registry {
	return &Registry{
		personas: make(map[string]*Persona),
	}
}

// Load scans a directory for persona .md files and populates the registry. SPEC B.3.2.
func (r *Registry) Load(dir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.dir = dir

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("personas directory not found", "dir", dir)
			r.personas = make(map[string]*Persona)
			return nil
		}
		return fmt.Errorf("stat personas dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("personas path is not a directory: %s", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read personas dir: %w", err)
	}

	newPersonas := make(map[string]*Persona)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		p, err := loadPersonaFile(path)
		if err != nil {
			slog.Warn("invalid persona file, skipping",
				"path", path,
				"error", err,
			)
			continue
		}

		// Validate name matches filename stem.
		stem := strings.TrimSuffix(entry.Name(), ".md")
		if p.Name != stem {
			slog.Warn("persona name does not match filename, skipping",
				"path", path,
				"name", p.Name,
				"expected", stem,
			)
			continue
		}

		if _, exists := newPersonas[p.Name]; exists {
			slog.Warn("duplicate persona name, skipping",
				"name", p.Name,
				"path", path,
			)
			continue
		}

		newPersonas[p.Name] = p
	}

	r.personas = newPersonas
	slog.Info("personas loaded", "count", len(newPersonas), "dir", dir)
	return nil
}

// Get returns a persona by name, or nil if not found.
func (r *Registry) Get(name string) *Persona {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.personas[name]
}

// List returns all loaded personas.
func (r *Registry) List() []*Persona {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Persona, 0, len(r.personas))
	for _, p := range r.personas {
		result = append(result, p)
	}
	return result
}

// Create writes a new persona file and adds it to the registry. SPEC B.10.3.
func (r *Registry) Create(p *Persona) error {
	if err := validatePersona(p); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.personas[p.Name]; exists {
		return fmt.Errorf("persona %q already exists", p.Name)
	}

	path := filepath.Join(r.dir, p.Name+".md")
	if err := writePersonaFile(path, p); err != nil {
		return fmt.Errorf("write persona file: %w", err)
	}

	p.SourcePath = path
	r.personas[p.Name] = p
	return nil
}

// Update modifies an existing persona definition. SPEC B.10.3.
func (r *Registry) Update(name string, p *Persona) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.personas[name]
	if !ok {
		return fmt.Errorf("persona %q not found", name)
	}

	p.Name = name
	p.SourcePath = existing.SourcePath

	if err := writePersonaFile(existing.SourcePath, p); err != nil {
		return fmt.Errorf("write persona file: %w", err)
	}

	r.personas[name] = p
	return nil
}

// Delete removes a persona definition file and entry. SPEC B.10.3.
func (r *Registry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.personas[name]
	if !ok {
		return fmt.Errorf("persona %q not found", name)
	}

	if err := os.Remove(existing.SourcePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove persona file: %w", err)
	}

	delete(r.personas, name)
	return nil
}

// loadPersonaFile parses a persona markdown file with YAML front matter.
func loadPersonaFile(path string) (*Persona, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	frontMatter, promptBody, err := splitPersonaFrontMatter(content)
	if err != nil {
		return nil, err
	}

	p := &Persona{
		SourcePath:     path,
		PromptTemplate: strings.TrimSpace(promptBody),
	}

	if name, ok := frontMatter["name"].(string); ok {
		p.Name = name
	} else {
		return nil, fmt.Errorf("missing required field: name")
	}

	if desc, ok := frontMatter["description"].(string); ok {
		p.Description = desc
	}

	if overrides, ok := frontMatter["overrides"].(map[string]any); ok {
		p.Overrides = parseOverrides(overrides)
	}

	return p, validatePersona(p)
}

// splitPersonaFrontMatter splits YAML front matter from the prompt body.
func splitPersonaFrontMatter(content string) (map[string]any, string, error) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return map[string]any{}, content, nil
	}

	rest := trimmed[3:]
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return map[string]any{}, content, nil
	}

	yamlPart := rest[:idx]
	promptPart := rest[idx+4:]

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(yamlPart), &raw); err != nil {
		return nil, "", fmt.Errorf("yaml parse error: %w", err)
	}
	if raw == nil {
		raw = map[string]any{}
	}

	return raw, promptPart, nil
}

func parseOverrides(raw map[string]any) PersonaOverrides {
	var o PersonaOverrides

	if agent, ok := raw["agent"].(map[string]any); ok {
		o.Agent = &AgentOverrides{}
		if v, ok := toIntPtr(agent["max_turns"]); ok {
			o.Agent.MaxTurns = v
		}
	}

	if codexMap, ok := raw["codex"].(map[string]any); ok {
		o.Codex = &CodexOverrides{}
		if v, ok := codexMap["approval_policy"].(string); ok {
			o.Codex.ApprovalPolicy = &v
		}
		if v, ok := codexMap["model"].(string); ok {
			o.Codex.Model = &v
		}
		if v, ok := toIntPtr(codexMap["turn_timeout_ms"]); ok {
			o.Codex.TurnTimeoutMs = v
		}
	}

	if tools, ok := raw["tools"].(map[string]any); ok {
		o.Tools = &ToolOverrides{}
		if allow, ok := tools["allow"].([]any); ok {
			for _, v := range allow {
				if s, ok := v.(string); ok {
					o.Tools.Allow = append(o.Tools.Allow, s)
				}
			}
		}
		if deny, ok := tools["deny"].([]any); ok {
			for _, v := range deny {
				if s, ok := v.(string); ok {
					o.Tools.Deny = append(o.Tools.Deny, s)
				}
			}
		}
	}

	return o
}

func validatePersona(p *Persona) error {
	if p.Name == "" {
		return fmt.Errorf("persona name is required")
	}
	if !validNameRe.MatchString(p.Name) {
		return fmt.Errorf("persona name %q must contain only [a-z0-9-]", p.Name)
	}
	if p.Overrides.Tools != nil && len(p.Overrides.Tools.Allow) > 0 && len(p.Overrides.Tools.Deny) > 0 {
		return fmt.Errorf("persona %q: tools.allow and tools.deny cannot both be present", p.Name)
	}
	return nil
}

func writePersonaFile(path string, p *Persona) error {
	var buf strings.Builder

	buf.WriteString("---\n")
	buf.WriteString(fmt.Sprintf("name: %s\n", p.Name))
	if p.Description != "" {
		buf.WriteString(fmt.Sprintf("description: %s\n", p.Description))
	}

	if p.Overrides.Agent != nil || p.Overrides.Codex != nil || p.Overrides.Tools != nil {
		overridesYAML, err := yaml.Marshal(map[string]any{"overrides": marshalOverrides(p.Overrides)})
		if err == nil {
			buf.Write(overridesYAML)
		}
	}

	buf.WriteString("---\n\n")
	if p.PromptTemplate != "" {
		buf.WriteString(p.PromptTemplate)
		buf.WriteString("\n")
	}

	return os.WriteFile(path, []byte(buf.String()), 0644)
}

func marshalOverrides(o PersonaOverrides) map[string]any {
	result := make(map[string]any)
	if o.Agent != nil {
		agent := make(map[string]any)
		if o.Agent.MaxTurns != nil {
			agent["max_turns"] = *o.Agent.MaxTurns
		}
		if len(agent) > 0 {
			result["agent"] = agent
		}
	}
	if o.Codex != nil {
		cdx := make(map[string]any)
		if o.Codex.ApprovalPolicy != nil {
			cdx["approval_policy"] = *o.Codex.ApprovalPolicy
		}
		if o.Codex.Model != nil {
			cdx["model"] = *o.Codex.Model
		}
		if o.Codex.TurnTimeoutMs != nil {
			cdx["turn_timeout_ms"] = *o.Codex.TurnTimeoutMs
		}
		if len(cdx) > 0 {
			result["codex"] = cdx
		}
	}
	if o.Tools != nil {
		tools := make(map[string]any)
		if len(o.Tools.Allow) > 0 {
			tools["allow"] = o.Tools.Allow
		}
		if len(o.Tools.Deny) > 0 {
			tools["deny"] = o.Tools.Deny
		}
		if len(tools) > 0 {
			result["tools"] = tools
		}
	}
	return result
}

func toIntPtr(v any) (*int, bool) {
	switch n := v.(type) {
	case int:
		return &n, true
	case int64:
		i := int(n)
		return &i, true
	case float64:
		i := int(n)
		return &i, true
	default:
		return nil, false
	}
}
