// Package workspace manages per-issue workspace directories and lifecycle hooks.
// SPEC Sections 9.1–9.5.
package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/siddhant2408/nimbus/internal/config"
	"github.com/siddhant2408/nimbus/internal/domain"
)

// Manager handles workspace creation, removal, and hook execution.
// It takes a config accessor function so it always reads the latest
// hot-reloaded configuration.
type Manager struct {
	config func() *config.Config
}

// NewManager creates a Manager that reads config from the provided accessor.
func NewManager(configFn func() *config.Config) *Manager {
	return &Manager{config: configFn}
}

// CreateForIssue creates or reuses the workspace directory for the given issue
// identifier. It returns a WorkspaceResult indicating the resolved path and
// whether the directory was newly created. If the directory is newly created
// and an after_create hook is configured, the hook is executed; failure is fatal.
// SPEC Section 9.2.
func (m *Manager) CreateForIssue(ctx context.Context, identifier string) (*domain.WorkspaceResult, error) {
	cfg := m.config()
	safeID := SafeIdentifier(identifier)
	workspacePath := filepath.Join(cfg.Workspace.Root, safeID)

	// Validate containment before any filesystem operations.
	if err := ValidateWorkspacePath(workspacePath, cfg.Workspace.Root); err != nil {
		return nil, fmt.Errorf("workspace path validation failed for %q: %w", identifier, err)
	}

	createdNow, err := ensureDir(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("workspace creation failed for %q: %w", identifier, err)
	}

	// Run after_create hook only for newly created workspaces.
	if createdNow {
		if err := m.RunAfterCreateHook(ctx, workspacePath); err != nil {
			// after_create failure is fatal — clean up the partially-created workspace.
			slog.Error("after_create hook failed, removing workspace",
				"workspace", workspacePath,
				"error", err,
			)
			_ = os.RemoveAll(workspacePath)
			return nil, fmt.Errorf("after_create hook failed for %q: %w", identifier, err)
		}
	}

	return &domain.WorkspaceResult{
		Path:       workspacePath,
		CreatedNow: createdNow,
	}, nil
}

// Remove deletes the workspace directory for the given identifier after running
// the before_remove hook (if configured). Hook failures are logged and ignored.
// SPEC Section 9.4.
func (m *Manager) Remove(ctx context.Context, identifier string) error {
	cfg := m.config()
	safeID := SafeIdentifier(identifier)
	workspacePath := filepath.Join(cfg.Workspace.Root, safeID)

	// Only validate and run hooks if the path actually exists.
	info, err := os.Stat(workspacePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to remove
		}
		return fmt.Errorf("workspace stat failed for %q: %w", identifier, err)
	}

	if info.IsDir() {
		if err := ValidateWorkspacePath(workspacePath, cfg.Workspace.Root); err != nil {
			return fmt.Errorf("workspace path validation failed for %q: %w", identifier, err)
		}
		m.RunBeforeRemoveHook(ctx, workspacePath)
	}

	if err := os.RemoveAll(workspacePath); err != nil {
		return fmt.Errorf("workspace removal failed for %q: %w", identifier, err)
	}

	slog.Info("Workspace removed", "identifier", identifier, "path", workspacePath)
	return nil
}

// RunBeforeRunHook executes the before_run hook script in the given workspace.
// Returns an error on failure or timeout; the caller should abort the attempt.
// SPEC Section 9.4: failure is fatal to the current run attempt.
func (m *Manager) RunBeforeRunHook(ctx context.Context, workspacePath string) error {
	cfg := m.config()
	if cfg.Hooks.BeforeRun == "" {
		return nil
	}
	return runHook(ctx, cfg, cfg.Hooks.BeforeRun, workspacePath, "before_run")
}

// RunAfterRunHook executes the after_run hook script in the given workspace.
// Failures are logged and ignored.
// SPEC Section 9.4: failure is logged and ignored.
func (m *Manager) RunAfterRunHook(ctx context.Context, workspacePath string) {
	cfg := m.config()
	if cfg.Hooks.AfterRun == "" {
		return
	}
	if err := runHook(ctx, cfg, cfg.Hooks.AfterRun, workspacePath, "after_run"); err != nil {
		slog.Warn("after_run hook failed (ignored)",
			"workspace", workspacePath,
			"error", err,
		)
	}
}

// RunAfterCreateHook executes the after_create hook script in the given workspace.
// Returns an error on failure or timeout; the caller should treat this as fatal.
// SPEC Section 9.4: failure is fatal to workspace creation.
func (m *Manager) RunAfterCreateHook(ctx context.Context, workspacePath string) error {
	cfg := m.config()
	if cfg.Hooks.AfterCreate == "" {
		return nil
	}
	return runHook(ctx, cfg, cfg.Hooks.AfterCreate, workspacePath, "after_create")
}

// RunBeforeRemoveHook executes the before_remove hook script in the given workspace.
// Failures are logged and ignored.
// SPEC Section 9.4: failure is logged and ignored.
func (m *Manager) RunBeforeRemoveHook(ctx context.Context, workspacePath string) {
	cfg := m.config()
	if cfg.Hooks.BeforeRemove == "" {
		return
	}
	if err := runHook(ctx, cfg, cfg.Hooks.BeforeRemove, workspacePath, "before_remove"); err != nil {
		slog.Warn("before_remove hook failed (ignored)",
			"workspace", workspacePath,
			"error", err,
		)
	}
}

// ensureDir makes sure the target path is a directory. If a non-directory file
// exists at the path, it is removed first. Returns true if the directory was
// newly created.
func ensureDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, nil // existing directory — reuse
		}
		// Exists but is not a directory — remove and recreate.
		if err := os.RemoveAll(path); err != nil {
			return false, fmt.Errorf("remove non-directory at workspace path: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return false, fmt.Errorf("mkdir workspace: %w", err)
	}
	return true, nil
}
