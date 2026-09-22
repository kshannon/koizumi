package schedule

import (
	"strings"
	"testing"
	"time"
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

func TestNextCheck(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 9, 22, h, m, 0, 0, time.Local) }
	if got := Next(at(10, 0)); !got.Equal(at(15, 0)) {
		t.Errorf("after 10:00 the next check is 15:00, got %v", got)
	}
	if got := Next(at(16, 30)); !got.Equal(at(9, 0).Add(24 * time.Hour)) {
		t.Errorf("after 16:30 the next check is 09:00 tomorrow, got %v", got)
	}
	if got := Next(at(9, 0)); !got.Equal(at(15, 0)) {
		t.Errorf("exactly at 09:00 the next check is 15:00, got %v", got)
	}
}
