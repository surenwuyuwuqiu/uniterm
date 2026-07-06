//go:build darwin

package main

import (
	"os/exec"
	"strings"

	"github.com/ys-ll/uniterm/backend/log"
)

// macBundleID is the app's bundle identifier. Wails derives the default
// identifier from wails.json's "name" as com.wails.<name>, and this project
// ships no custom build/darwin/Info.plist, so it resolves to com.wails.uniTerm.
const macBundleID = "com.wails.uniTerm"

// configureMacKeyRepeat disables macOS's press-and-hold accent picker for this
// app's bundle only, so that holding a key down produces continuous key-repeat
// input in the terminal (matching Windows behaviour) instead of popping up the
// system accent/variant character picker.
//
// This works around the long-standing upstream xterm.js issue on macOS
// (xtermjs/xterm.js#265, #4385): macOS intercepts a held key at the OS level to
// show the accent popup, which suppresses the key-repeat event stream the
// terminal expects. Scoping the preference to this bundle identifier leaves
// every other application's behaviour untouched.
//
// The setting is written once (only when not already disabled) and persists
// across runs, so it is a no-op on subsequent launches. It runs asynchronously
// to avoid adding latency to startup.
func (a *App) configureMacKeyRepeat() {
	go func() {
		// Skip the write if it's already disabled to avoid churning the
		// preferences daemon on every launch. `defaults read` prints "0" for a
		// false boolean.
		if out, err := exec.Command("defaults", "read", macBundleID, "ApplePressAndHoldEnabled").Output(); err == nil {
			if strings.TrimSpace(string(out)) == "0" {
				return
			}
		}

		if err := exec.Command("defaults", "write", macBundleID, "ApplePressAndHoldEnabled", "-bool", "false").Run(); err != nil {
			log.Writef("configureMacKeyRepeat: failed to disable ApplePressAndHoldEnabled: %v", err)
			return
		}
		log.Writef("configureMacKeyRepeat: disabled ApplePressAndHoldEnabled for %s (key-repeat enabled)", macBundleID)
	}()
}
