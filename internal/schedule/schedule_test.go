package schedule

import (
	"strings"
	"testing"
)

func TestPlistRunsCheckThroughTheLoginShell(t *testing.T) {
	// The job must run through the user's login shell so it sees the same environment the
	// user does. Homebrew keeps tap-trust records under $XDG_CONFIG_HOME, which the user's
	// .zshenv sets and launchd does not; without it brew silently drops formulae from taps.
	p := Plist("/bin/zsh", "/Users/kyle/go/bin/koizumi", "/Users/kyle/Library/Logs/koizumi.log")
	for _, want := range []string{
		"<string>" + Label + "</string>",
		"<string>/bin/zsh</string>",
		"<string>-lc</string>",
		"<string>'/Users/kyle/go/bin/koizumi' check</string>",
		"<key>StartCalendarInterval</key>",
		"<integer>9</integer>", "<integer>15</integer>",
		"<key>RunAtLoad</key>",
		"<key>PATH</key>",
		"/opt/homebrew/bin:", // launchd's default PATH has no Homebrew: the old doctor never found brew
		"<string>/Users/kyle/Library/Logs/koizumi.log</string>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("plist is missing %q", want)
		}
	}
	if !strings.HasPrefix(p, "<?xml") || !strings.HasSuffix(strings.TrimSpace(p), "</plist>") {
		t.Error("not a complete plist document")
	}
}
