package check

import (
	"path/filepath"
	"strings"
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

// Motd is one calm sentence, without the speaker (the command line adds "✨ koizumi:").
// It counts categories, not items: five things are waiting, not 128.
func TestMotd(t *testing.T) {
	r := fixture()
	now := r.When.Add(2 * time.Hour)
	if got, want := Motd(r, now), "5 things are waiting."; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	one := Report{When: r.When, Outdated: []outdated.Probe{{Source: "macOS", Status: "outdated", Items: []outdated.Item{{Name: "Safari"}}}}, Brewfile: brewfile.Probe{Status: "ok"}}
	if got, want := Motd(one, now), "1 thing is waiting."; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	failed := fixture()
	failed.Outdated[0].Status, failed.Outdated[0].Note = "error", "brew exploded"
	if got, want := Motd(failed, now), "5 things are waiting; a check failed."; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	// nothing to say when everything is fine
	clean := Report{When: r.When, Brewfile: brewfile.Probe{Status: "ok"}}
	if got := Motd(clean, now); got != "" {
		t.Errorf("clean report should be silent, got %q", got)
	}
	// a stale cache must speak up even when the last report was clean
	if got, want := Motd(clean, r.When.Add(3*24*time.Hour)), "last check was 3d ago; is the schedule alive? koizumi setup"; got != want {
		t.Errorf("stale: got  %q\nwant %q", got, want)
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

func TestSectionsAlwaysShowEveryCategoryInAFixedOrder(t *testing.T) {
	r := fixture() // macOS outdated, App Store skipped, git drift, Brewfile drift, 1 unknown app
	s := Sections(r)
	var names, levels []string
	for _, x := range s {
		names, levels = append(names, x.Name), append(levels, x.Level)
	}
	wantNames := []string{"macOS", "Homebrew", "App Store", "dotfiles", "apps"}
	wantLevels := []string{"warn", "warn", "skipped", "warn", "warn"}
	for i := range wantNames {
		if i >= len(names) || names[i] != wantNames[i] || levels[i] != wantLevels[i] {
			t.Fatalf("got %v %v, want %v %v", names, levels, wantNames, wantLevels)
		}
	}
	// Homebrew carries the Brewfile fact as its own line, so "behind" and "missing" never blur
	if !contains(s[1].Text, "117 behind") || !contains(strings.Join(s[1].Lines, "\n"), "Brewfile: 3 missing, 2 extra") {
		t.Errorf("Homebrew section = %+v", s[1])
	}
	// what is behind is named on the line (first three, "+N more"), and the fix line is just
	// the command: no "(koizumi outdated for the list)" aside
	// commands carry a leading arrow so the printer can tell them from facts
	if !contains(s[1].Text, "117 behind: ") || !contains(s[1].Text, "+114 more") || s[1].Lines[0] != "→ brew upgrade" {
		t.Errorf("Homebrew section = %+v", s[1])
	}
	if !contains(strings.Join(s[1].Lines, "\n"), "→ koizumi brew") {
		t.Errorf("Brewfile drift needs its command: %+v", s[1].Lines)
	}
	as := Sections(Report{Outdated: []outdated.Probe{{Source: "App Store", Status: "outdated",
		Items: []outdated.Item{{Name: "Keynote"}, {Name: "Numbers"}, {Name: "Pages"}}}}})[2]
	if as.Text != "3 behind: Keynote, Numbers, Pages" || len(as.Lines) != 1 || as.Lines[0] != "→ mas upgrade" {
		t.Errorf("App Store section = %+v", as)
	}
	// a note reads as an aside in parentheses, never a middot
	mac := Sections(Report{Outdated: []outdated.Probe{{Source: "macOS", Status: "ok", Note: "last checked 7h ago"}}})[0]
	if mac.Text != "current (last checked 7h ago)" {
		t.Errorf("macOS ok text = %q", mac.Text)
	}
	// the overrides name apps this machine does not have: said as a fact, with the command
	ap := appsSection(Apps{Total: 17, Unknown: []string{}, NotInstalled: []string{"Anki", "Steam", "pgAdmin 4", "texstudio"}})
	if ap.Level != "ok" || len(ap.Lines) != 1 || ap.Lines[0] != "not installed here, but in the overrides: Anki, Steam, pgAdmin 4, +1 more (koizumi apps)" {
		t.Errorf("apps section = %+v", ap)
	}
	// a clean report says so in every section
	c := Report{
		Outdated: []outdated.Probe{{Source: "Homebrew", Status: "ok"}, {Source: "App Store", Status: "ok"}, {Source: "macOS", Status: "ok"}},
		Dotfiles: []dotfiles.Probe{{Source: "chezmoi", Status: "ok"}, {Source: "git", Status: "ok", Branch: "main", Upstream: true}},
		Brewfile: brewfile.Probe{Status: "ok"}, Apps: Apps{Total: 16},
	}
	for _, x := range Sections(c) {
		if x.Level != "ok" {
			t.Errorf("%s should be ok in a clean report, got %s: %s", x.Name, x.Level, x.Text)
		}
	}
	if got := Sections(c)[3].Text; got != "in sync with the repo, repo in sync with GitHub" {
		t.Errorf("clean dotfiles text = %q", got)
	}
}

// The koizumi line always says which commit is running and when it was made; behind, it
// names both sides and the install command.
func TestKoizumiSectionNamesTheCommits(t *testing.T) {
	r := Report{Outdated: []outdated.Probe{{Source: "koizumi", Status: "ok", Note: "running 700c1c1 from 2026-09-22 18:10"}}}
	s := Sections(r)
	k := s[len(s)-1]
	if k.Name != "koizumi" || k.Level != "ok" || k.Text != "current (running 700c1c1 from 2026-09-22 18:10)" {
		t.Errorf("current: %+v", k)
	}
	r.Outdated[0] = outdated.Probe{Source: "koizumi", Status: "outdated", Note: "running 473f944 from 2026-09-22 17:57",
		Items: []outdated.Item{{Name: "koizumi", Installed: "473f944 from 2026-09-22 17:57", Latest: "700c1c1 from 2026-09-22 18:10",
			Fix: "go install github.com/kshannon/koizumi@main"}}}
	s = Sections(r)
	k = s[len(s)-1]
	if k.Level != "warn" || k.Text != "behind: running 473f944 from 2026-09-22 17:57, main is 700c1c1 from 2026-09-22 18:10" {
		t.Errorf("behind: %+v", k)
	}
	if len(k.Lines) != 1 || !contains(k.Lines[0], "go install github.com/kshannon/koizumi@main") {
		t.Errorf("behind lines: %v", k.Lines)
	}
}
