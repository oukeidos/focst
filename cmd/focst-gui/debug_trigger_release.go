//go:build !debug

package main

func debugStateForPath(_ string) (AppState, bool) {
	return StateIdle, false
}
