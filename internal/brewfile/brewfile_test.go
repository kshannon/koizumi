package brewfile

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	in := `# a comment
tap "heroku/brew"
brew "openssl@3"
brew "postgresql@14", restart_service: :changed   # keep
brew "heroku/brew/heroku"
cask "raycast"
mas "Things3", id: 904280696
vscode "some.extension"

`
	got := Parse(strings.NewReader(in))
	want := []Entry{
		{"tap", "heroku/brew"}, {"brew", "openssl@3"}, {"brew", "postgresql@14"},
		{"brew", "heroku/brew/heroku"}, {"cask", "raycast"}, {"mas", "Things3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestCompare(t *testing.T) {
	entries := []Entry{
		{"brew", "a"}, {"brew", "b"}, {"brew", "heroku/brew/heroku"},
		{"cask", "x"}, {"tap", "t1"},
	}
	installed := Installed{
		Formulae: []string{"c", "heroku", "a"},
		Casks:    []string{"y"},
		Taps:     []string{"t2", "t1"},
	}
	got := Compare(entries, installed)
	want := Diff{
		MissingFormulae: []string{"b"}, ExtraFormulae: []string{"c"},
		MissingCasks: []string{"x"}, ExtraCasks: []string{"y"},
		MissingTaps: []string{}, ExtraTaps: []string{"t2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if !got.Any() {
		t.Error("Any() should be true when something differs")
	}
	if (Diff{}).Any() {
		t.Error("an empty diff should not be Any()")
	}
}

func TestFixesExplainBothReadingsOfMissingAndExtra(t *testing.T) {
	d := Diff{
		MissingFormulae: []string{"b"}, MissingCasks: []string{"x"},
		ExtraFormulae: []string{"c", "d"}, ExtraCasks: []string{"y"}, ExtraTaps: []string{"t2"},
	}
	files := []string{"/dots/brew/Brewfile.common", "/dots/brew/Brewfile.box"}
	got := Fixes(d, files)
	want := []Fix{
		{When: "missing, and you still want it? reinstall it",
			Commands: []string{"brew bundle --no-upgrade --file=/dots/brew/Brewfile.common", "brew bundle --no-upgrade --file=/dots/brew/Brewfile.box"}},
		{When: "missing because you removed it on purpose? take it out of the Brewfile",
			Commands: []string{"koizumi brew edit"}},
		{When: "extra, and you want to keep it? add it to the Brewfile",
			Commands: []string{"koizumi brew edit"}},
		{When: "extra, and you do not want it? remove it (koizumi never removes anything)",
			Commands: []string{"brew uninstall c d", "brew uninstall --cask y", "brew untap t2"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestFixesOnlyMentionWhatIsWrong(t *testing.T) {
	files := []string{"/dots/brew/Brewfile.common"}
	if got := Fixes(Diff{}, files); len(got) != 0 {
		t.Errorf("nothing wrong, nothing to fix; got %+v", got)
	}
	got := Fixes(Diff{ExtraCasks: []string{"y"}}, files)
	if len(got) != 2 || !strings.HasPrefix(got[0].When, "extra") || !strings.HasPrefix(got[1].When, "extra") {
		t.Errorf("only extras: want the two extra fixes, got %+v", got)
	}
	if !reflect.DeepEqual(got[1].Commands, []string{"brew uninstall --cask y"}) {
		t.Errorf("one extra cask: want just its uninstall, got %+v", got[1].Commands)
	}
}
