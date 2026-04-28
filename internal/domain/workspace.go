package domain

// WorkspaceResult is the result of creating or reusing a workspace directory.
type WorkspaceResult struct {
	Path       string
	CreatedNow bool
}
