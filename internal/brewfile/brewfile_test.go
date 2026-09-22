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
