package hooks

import "walkline/internal/constants"

// Hook status values
type HookStatus int

const (
	HookFresh HookStatus = iota
	HookMerged
	HookNoOp
)

// PostCommitHook returns the post-commit hook content
func PostCommitHook() string {
	return constants.PostCommitHookContent
}

// PrePushHook returns the full pre-push hook content
func PrePushHook() string {
	return `#!/bin/sh
# walkline pre-push hook
# Automatically installed by walkline
` + constants.PrePushHookBody
}

// PrePushHookMarker returns the marker string for pre-push hooks
func PrePushHookMarker() string {
	return constants.PrePushHookMarker
}
