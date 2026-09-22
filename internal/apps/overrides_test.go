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

func TestParseOverridesRejectsBadLines(t *testing.T) {
	for _, in := range []string{"Steam = sometimes", "Steam self", "= self"} {
		if _, err := ParseOverrides(strings.NewReader(in)); err == nil {
			t.Errorf("%q: expected an error", in)
		} else if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("%q: error should name the line, got: %v", in, err)
		}
	}
}
