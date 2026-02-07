//go:build debug

package main

import "strings"

const (
	debugCancelToken  = "debug_cancel_3f7a9b1c5d2e4f8a9c7b6d5e4f3a2b1c"
	debugFailureToken = "debug_failure_3f7a9b1c5d2e4f8a9c7b6d5e4f3a2b1c"
	debugPartialToken = "debug_partial_3f7a9b1c5d2e4f8a9c7b6d5e4f3a2b1c"
	debugSuccessToken = "debug_success_3f7a9b1c5d2e4f8a9c7b6d5e4f3a2b1c"
)

func debugStateForPath(path string) (AppState, bool) {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, debugCancelToken):
		return StateCanceled, true
	case strings.Contains(lower, debugFailureToken):
		return StateFailure, true
	case strings.Contains(lower, debugPartialToken):
		return StatePartialSuccess, true
	case strings.Contains(lower, debugSuccessToken):
		return StateSuccess, true
	default:
		return StateIdle, false
	}
}
