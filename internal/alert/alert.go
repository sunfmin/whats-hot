// Package alert raises a user-facing macOS notification when a Stall is detected on an
// Observed Transfer the tool cannot itself resume (see docs/CONTEXT.md: Alert).
package alert

import (
	"os/exec"
	"strings"
)

// Notify posts a macOS Notification Center banner via osascript. Errors are returned but
// callers typically ignore them — a failed banner must never break monitoring.
func Notify(title, message string) error {
	script := "display notification \"" + escape(message) + "\" with title \"" + escape(title) + "\""
	return exec.Command("osascript", "-e", script).Run()
}

// escape makes a string safe to embed inside an AppleScript double-quoted literal.
func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}
