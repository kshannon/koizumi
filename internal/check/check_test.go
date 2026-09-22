package check

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kshannon/koizumi/internal/brewfile"
	"github.com/kshannon/koizumi/internal/dotfiles"
	"github.com/kshannon/koizumi/internal/outdated"
)

func fixture() Report {
	return Report{
		When: time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC),
		Host: "yukibot",
		Outdated: []outdated.Probe{
			{Source: "Homebrew", Status: "outdated", Note: "index refreshed 4d ago", Items: make([]outdated.Item, 117)},
			{Source: "App Store", Status: "skipped", Note: "install mas", Items: []outdated.Item{}},
			{Source: "macOS", Status: "outdated", Items: []outdated.Item{{Name: "macOS 27"}, {Name: "Safari"}, {Name: "macOS Tahoe 26.7"}}},
		},
		Dotfiles: []dotfiles.Probe{
			{Source: "chezmoi", Status: "skipped", Note: "not set up", Entries: []dotfiles.Entry{}},
			{Source: "git", Status: "drift", Fix: "commit", Entries: []dotfiles.Entry{{Status: "M", Path: "a"}, {Status: "M", Path: "b"}}},
		},
		Brewfile: brewfile.Probe{Source: "Brewfile", Status: "drift", Diff: brewfile.Diff{
			MissingFormulae: []string{"mas", "postgresql@15", "postgresql@17"}, ExtraFormulae: []string{"node@18"}, ExtraTaps: []string{"homebrew/services"}}},
		Apps: Apps{Total: 55, Unknown: []string{"Anki"}},
	}
}

func TestAttentionIsOrderedBySeverity(t *testing.T) {
	items := Attention(fixture())
	var sources []string
	for _, it := range items {
		sources = append(sources, it.Source)
	}
	want := []string{"macOS", "git", "Brewfile", "Homebrew", "apps"}
	if len(sources) != len(want) {
		t.Fatalf("got %v, want %v", sources, want)
	}
	for i := range want {
		if sources[i] != want[i] {
			t.Fatalf("got %v, want %v", sources, want)
		}
	}
	if items[0].Text != "3 updates: macOS 27, Safari, macOS Tahoe 26.7" {
		t.Errorf("macOS text = %q", items[0].Text)
	}
	if items[2].Text != "3 missing, 2 extra" {
		t.Errorf("Brewfile text = %q", items[2].Text)
	}
}

func TestErrorsComeFirstAndSkippedAreNotAttention(t *testing.T) {
	r := fixture()
	r.Outdated[0].Status, r.Outdated[0].Note = "error", "brew exploded"
	items := Attention(r)
	if items[0].Source != "Homebrew" || items[0].Level != "error" {
		t.Fatalf("an error must sort first, got %+v", items[0])
	}
	for _, it := range items {
		if it.Level == "skipped" {
			t.Errorf("skipped probes are not attention: %+v", it)
		}
	}
	if n := len(Skipped(r)); n != 2 {
		t.Errorf("Skipped() = %d, want 2 (App Store, chezmoi)", n)
	}
}

func TestMotd(t *testing.T) {
	r := fixture()
	now := r.When.Add(2 * time.Hour)
	if got, want := Motd(r, now), "koizumi ▲ macOS 3 · dotfiles 2 · Brewfile 5 · Homebrew 117 · apps 1 → koizumi"; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	// nothing to say when everything is fine
	clean := Report{When: r.When, Brewfile: brewfile.Probe{Status: "ok"}}
	if got := Motd(clean, now); got != "" {
		t.Errorf("clean report should be silent, got %q", got)
	}
	// a stale cache must speak up even when the last report was clean
	if got := Motd(clean, r.When.Add(3*24*time.Hour)); got == "" || !contains(got, "last check 3d ago") {
		t.Errorf("stale report should say so, got %q", got)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "report.json")
	r := fixture()
	if err := Save(r, path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.When.Equal(r.When) || got.Host != "yukibot" || len(got.Outdated[0].Items) != 117 || got.Apps.Unknown[0] != "Anki" {
		t.Errorf("round trip lost data: %+v", got)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
