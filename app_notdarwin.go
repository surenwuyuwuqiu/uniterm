//go:build !darwin

package main

// configureMacKeyRepeat is a no-op on non-macOS platforms; the press-and-hold
// accent picker only exists on macOS. See app_darwin.go for details.
func (a *App) configureMacKeyRepeat() {}
