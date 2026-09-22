package apps

import (
	"strings"
	"testing"
)

func TestParseOverrides(t *testing.T) {
	in := `# apps koizumi cannot work out by itself
Microsoft Teams = self       # Microsoft AutoUpdate
Steam           = self
Safari          = macos      # comes with macOS

Anki = manual   # download from the website
`
	got, err := ParseOverrides(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Override{
		"Microsoft Teams": {Self, "Microsoft AutoUpdate"},
		"Steam":           {Self, ""},
		"Safari":          {MacOS, "comes with macOS"},
		"Anki":            {Manual, "download from the website"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d overrides, want %d: %+v", len(got), len(want), got)
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: got %+v, want %+v", name, got[name], w)
		}
	}
}

// The overrides file is shared between machines through the dotfiles, so an entry with no
// matching app here is an app the other machine has: "how do I get it?" is the note.
func TestNotInstalledListsOverridesWithNoAppHere(t *testing.T) {
	overrides := map[string]Override{
		"Anki":  {Updater: Manual, Note: "download from apps.ankiweb.net"},
		"Steam": {Updater: Self, Note: "updates itself on launch"},
		"Kap":   {Updater: Self, Note: "Sparkle"},
	}
	installed := []App{{Name: "Kap"}, {Name: "Ghostty"}}
	got := NotInstalled(overrides, installed)
	if len(got) != 2 || got[0].Name != "Anki" || got[0].Note != "download from apps.ankiweb.net" || got[1].Name != "Steam" {
		t.Errorf("got %+v", got)
	}
	if n := len(NotInstalled(map[string]Override{}, installed)); n != 0 {
		t.Errorf("no overrides: got %d", n)
	}
}

func TestParseOverridesRejectsBadLines(t *testing.T) {
	for _, in := range []string{"Steam = sometimes", "Steam self", "= self"} {
		if _, err := ParseOverrides(strings.NewReader(in)); err == nil {
			t.Errorf("%q: expected an error", in)
		} else if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("%q: error should name the line, got: %v", in, err)
		}
	}
}
