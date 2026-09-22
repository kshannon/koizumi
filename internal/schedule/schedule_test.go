package schedule

import (
	"strings"
	"testing"
)

func TestPlistNamesTheBinaryTheScheduleAndAnExplicitPath(t *testing.T) {
	p := Plist("/Users/kyle/go/bin/koizumi", "/Users/kyle/Library/Logs/koizumi.log")
	for _, want := range []string{
		"<string>" + Label + "</string>",
		"<string>/Users/kyle/go/bin/koizumi</string>",
		"<string>check</string>",
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
