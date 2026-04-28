package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// unsafeChars matches any character not in [A-Za-z0-9._-].
var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._\-]`)

// SafeIdentifier replaces characters outside [A-Za-z0-9._-] with underscores.
// If identifier is empty, it defaults to "issue".
// SPEC Section 9.5 Invariant 3.
func SafeIdentifier(identifier string) string {
	if identifier == "" {
		identifier = "issue"
	}
	return unsafeChars.ReplaceAllString(identifier, "_")
}

// ValidateWorkspacePath ensures workspacePath is strictly under workspaceRoot
// after resolving symlinks segment-by-segment. This prevents symlink escape
// attacks where an intermediate symlink component points outside the root.
// SPEC Section 9.5 Invariant 2.
func ValidateWorkspacePath(workspacePath, workspaceRoot string) error {
	absWorkspace, err := filepath.Abs(workspacePath)
	if err != nil {
		return fmt.Errorf("workspace path resolution failed: %w", err)
	}
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return fmt.Errorf("workspace root resolution failed: %w", err)
	}

	canonicalWorkspace, err := canonicalize(absWorkspace)
	if err != nil {
		return fmt.Errorf("workspace path unreadable: %w", err)
	}
	canonicalRoot, err := canonicalize(absRoot)
	if err != nil {
		return fmt.Errorf("workspace root unreadable: %w", err)
	}

	if canonicalWorkspace == canonicalRoot {
		return fmt.Errorf("workspace path equals workspace root: %s", canonicalRoot)
	}

	// The canonical workspace must have the canonical root as a strict directory prefix.
	if strings.HasPrefix(canonicalWorkspace, canonicalRoot+string(filepath.Separator)) {
		return nil
	}

	// If the un-resolved expanded path appeared to be under root but canonical is not,
	// that indicates a symlink escape.
	if strings.HasPrefix(absWorkspace, absRoot+string(filepath.Separator)) {
		return fmt.Errorf("workspace symlink escape detected: %s resolves outside root %s", absWorkspace, canonicalRoot)
	}

	return fmt.Errorf("workspace path %s is outside root %s", canonicalWorkspace, canonicalRoot)
}

// canonicalize resolves a path segment-by-segment, following symlinks at each
// step. This catches symlink escapes that would be missed by resolving the
// entire path at once (since the OS would stop at the first missing segment).
func canonicalize(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("canonicalize requires absolute path, got %q", path)
	}

	vol := filepath.VolumeName(cleaned)
	rest := cleaned[len(vol):]

	// Split into segments. On Unix the first element will be "/".
	segments := splitPath(rest)
	if len(segments) == 0 {
		return vol + string(filepath.Separator), nil
	}

	// Start from root.
	resolved := vol + string(filepath.Separator)

	for i, seg := range segments {
		if seg == string(filepath.Separator) || seg == "" {
			continue
		}

		candidate := filepath.Join(resolved, seg)

		info, err := os.Lstat(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				// Path does not exist yet. Append remaining segments
				// without further resolution and return.
				remaining := append([]string{seg}, segments[i+1:]...)
				for _, r := range remaining {
					if r != string(filepath.Separator) && r != "" {
						resolved = filepath.Join(resolved, r)
					}
				}
				return resolved, nil
			}
			return "", err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(candidate)
			if err != nil {
				return "", fmt.Errorf("readlink %s: %w", candidate, err)
			}

			// Resolve relative symlink targets against the current resolved dir.
			if !filepath.IsAbs(target) {
				target = filepath.Join(resolved, target)
			}
			target = filepath.Clean(target)

			// Recursively canonicalize the symlink target with remaining segments appended.
			remaining := segments[i+1:]
			remainingParts := make([]string, 0, len(remaining))
			for _, r := range remaining {
				if r != string(filepath.Separator) && r != "" {
					remainingParts = append(remainingParts, r)
				}
			}
			fullTarget := target
			for _, rp := range remainingParts {
				fullTarget = filepath.Join(fullTarget, rp)
			}
			return canonicalize(fullTarget)
		}

		resolved = candidate
	}

	return resolved, nil
}

// splitPath breaks a path into its individual components.
func splitPath(path string) []string {
	var segments []string
	for {
		dir, file := filepath.Split(path)
		if file != "" {
			segments = append([]string{file}, segments...)
		}
		// Remove trailing separator from dir.
		dir = strings.TrimRight(dir, string(filepath.Separator))
		if dir == "" || dir == path {
			break
		}
		path = dir
	}
	return segments
}
