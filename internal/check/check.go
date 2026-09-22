// Package check runs every probe, decides what needs attention, and caches the result so
// a new terminal can show one line without running anything.
package check

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kshannon/koizumi/internal/apps"
	"github.com/kshannon/koizumi/internal/brewfile"
	"github.com/kshannon/koizumi/internal/dotfiles"
	"github.com/kshannon/koizumi/internal/outdated"
	"github.com/kshannon/koizumi/internal/when"
)

// Stale is how old a cached report may be before the terminal line says so.
// The schedule runs twice a day; a laptop can sleep through both.
const Stale = 36 * time.Hour

// Report is one run of every probe.
type Report struct {
	When     time.Time        `json:"when"`
	Host     string           `json:"host"`
	Outdated []outdated.Probe `json:"outdated"`
	Dotfiles []dotfiles.Probe `json:"dotfiles"`
	Brewfile brewfile.Probe   `json:"brewfile"`
	Apps     Apps             `json:"apps"`
}

// Apps is the short form of the app scan: how many, which have no known updater, and which
// apps the shared overrides file names that this machine does not have.
type Apps struct {
	Total        int      `json:"total"`
	Unknown      []string `json:"unknown"`
	NotInstalled []string `json:"not_installed"`
}

// Options say where to look.
type Options struct {
	Repo      string
	Brewfiles []string
	AppsDir   string
	Overrides string
}

// Run executes every probe now. The probes are independent (and mostly waiting on brew,
// git or the network), so they run concurrently.
func Run(o Options) Report {
	host, _ := os.Hostname()
	r := Report{When: time.Now(), Host: strings.TrimSuffix(host, ".local"), Apps: Apps{Unknown: []string{}, NotInstalled: []string{}}}
	var wg sync.WaitGroup
	wg.Add(4)
	go func() { defer wg.Done(); r.Outdated = outdated.All().Probes }()
	go func() { defer wg.Done(); r.Dotfiles = dotfiles.All(o.Repo).Probes }()
	go func() { defer wg.Done(); r.Brewfile = brewfile.Check(o.Brewfiles) }()
	go func() {
		defer wg.Done()
		casks, _ := apps.LoadCasks()
		overrides, _ := apps.LoadOverrides(o.Overrides)
		if list, err := apps.Scan(o.AppsDir, casks, overrides); err == nil {
			r.Apps.Total = len(list)
			for _, a := range list {
				if a.Updater == apps.Unknown {
					r.Apps.Unknown = append(r.Apps.Unknown, a.Name)
				}
			}
			for _, m := range apps.NotInstalled(overrides, list) {
				r.Apps.NotInstalled = append(r.Apps.NotInstalled, m.Name)
			}
		}
	}()
	wg.Wait()
	return r
}

// Item is one line of the dashboard.
type Item struct {
	Level  string `json:"level"`  // error, warn, skipped
	Source string `json:"source"` // the probe
	Label  string `json:"label"`  // the short name used in the terminal line
	Count  int    `json:"count"`
	Text   string `json:"text"`
	Fix    string `json:"fix,omitempty"`
	rank   int
}

// Attention lists what needs doing, most important first: failed checks, then macOS,
// dotfiles, the Brewfile, Homebrew and the App Store, then apps with no known updater.
// Skipped probes are not attention; see Skipped.
func Attention(r Report) []Item {
	var items []Item
	add := func(it Item) { items = append(items, it) }
	errorOf := func(source, note string) Item {
		return Item{Level: "error", Source: source, Label: source, Count: 1, Text: note, rank: 0}
	}
	for _, p := range r.Outdated {
		switch p.Status {
		case "error":
			add(errorOf(p.Source, p.Note))
		case "outdated":
			it := Item{Level: "warn", Source: p.Source, Label: p.Source, Count: len(p.Items)}
			switch p.Source {
			case "macOS":
				it.Text, it.Fix, it.rank = fmt.Sprintf("%d updates: %s", len(p.Items), names(p.Items, 3)), p.Items[0].Fix, 1
			case "App Store":
				it.Text, it.Fix, it.rank = fmt.Sprintf("%d behind: %s", len(p.Items), names(p.Items, 3)), "mas upgrade", 4
			default:
				it.Text, it.Fix, it.rank = fmt.Sprintf("%d behind: %s", len(p.Items), names(p.Items, 3)), "brew upgrade", 4
				if p.Note != "" {
					it.Text += " · " + p.Note
				}
			}
			add(it)
		}
	}
	for _, p := range r.Dotfiles {
		switch p.Status {
		case "error":
			add(errorOf(p.Source, p.Note))
		case "drift":
			it := Item{Level: "warn", Source: p.Source, Label: "dotfiles", Fix: p.Fix, rank: 2}
			if p.Source == "git" {
				var parts []string
				if n := len(p.Entries); n > 0 {
					parts = append(parts, fmt.Sprintf("%d uncommitted", n))
				}
				if p.Ahead > 0 {
					parts = append(parts, fmt.Sprintf("%d to push", p.Ahead))
				}
				if p.Behind > 0 {
					parts = append(parts, fmt.Sprintf("%d to pull", p.Behind))
				}
				it.Count = len(p.Entries) + p.Ahead + p.Behind
				it.Text = strings.Join(parts, ", ") + " in " + p.Repo
			} else {
				it.Count, it.Text = len(p.Entries), p.Note
			}
			add(it)
		}
	}
	switch r.Brewfile.Status {
	case "error":
		add(errorOf("Brewfile", r.Brewfile.Note))
	case "drift":
		d := r.Brewfile.Diff
		missing := len(d.MissingFormulae) + len(d.MissingCasks) + len(d.MissingTaps)
		extra := len(d.ExtraFormulae) + len(d.ExtraCasks) + len(d.ExtraTaps)
		add(Item{Level: "warn", Source: "Brewfile", Label: "Brewfile", Count: missing + extra,
			Text: fmt.Sprintf("%d missing, %d extra", missing, extra), Fix: "koizumi brew", rank: 3})
	}
	if n := len(r.Apps.Unknown); n > 0 {
		add(Item{Level: "warn", Source: "apps", Label: "apps", Count: n,
			Text: fmt.Sprintf("%d app(s) with no known updater: %s", n, strings.Join(first(r.Apps.Unknown, 5), ", ")),
			Fix:  "name them in the overrides file  (koizumi apps --help)", rank: 5})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].rank < items[j].rank })
	return items
}

// Section is one category on the dashboard, always shown, with its own mark.
type Section struct {
	Level string   `json:"level"` // ok, warn, error, skipped
	Name  string   `json:"name"`
	Text  string   `json:"text"`
	Lines []string `json:"lines,omitempty"` // fixes and secondary facts, shown indented
}

// Sections renders the report as fixed categories: macOS, Homebrew (with the Brewfile fact
// as its own line, so "behind" and "missing" never blur), App Store, dotfiles, apps, and
// koizumi itself when that probe ran.
func Sections(r Report) []Section {
	byName := map[string]outdated.Probe{}
	for _, p := range r.Outdated {
		byName[p.Source] = p
	}
	var s []Section
	s = append(s, updaterSection(byName["macOS"], "macOS", "updates"))
	hb := updaterSection(byName["Homebrew"], "Homebrew", "behind")
	switch r.Brewfile.Status {
	case "ok":
		hb.Lines = append(hb.Lines, "Brewfile: everything listed is installed")
	case "drift":
		d := r.Brewfile.Diff
		hb.Lines = append(hb.Lines, fmt.Sprintf("Brewfile: %d missing, %d extra",
			len(d.MissingFormulae)+len(d.MissingCasks)+len(d.MissingTaps), len(d.ExtraFormulae)+len(d.ExtraCasks)+len(d.ExtraTaps)), "→ koizumi brew")
		hb.Level = worse(hb.Level, "warn")
	case "error":
		hb.Lines = append(hb.Lines, "Brewfile: "+r.Brewfile.Note)
		hb.Level = "error"
	}
	s = append(s, hb, updaterSection(byName["App Store"], "App Store", "behind"), dotfilesSection(r.Dotfiles), appsSection(r.Apps))
	if p, ok := byName["koizumi"]; ok {
		s = append(s, updaterSection(p, "koizumi", "behind"))
	}
	return s
}

func updaterSection(p outdated.Probe, name, noun string) Section {
	sec := Section{Name: name}
	switch p.Status {
	// Lines that start with "→ " are commands; the printer shows them as such. The rest are facts.
	case "ok":
		sec.Level, sec.Text = "ok", "current"
		if p.Note != "" { // e.g. which koizumi commit is running, when macOS last checked
			sec.Text += " (" + p.Note + ")"
		}
	case "outdated":
		sec.Level = "warn"
		sec.Text = fmt.Sprintf("%d %s: %s", len(p.Items), noun, names(p.Items, 3))
		switch name {
		case "Homebrew":
			sec.Lines = append(sec.Lines, "→ brew upgrade")
		case "App Store":
			sec.Lines = append(sec.Lines, "→ mas upgrade")
		case "koizumi": // one item, both sides named: what runs here and what main is
			it := p.Items[0]
			sec.Text = "behind: running " + it.Installed + ", main is " + it.Latest
			sec.Lines = append(sec.Lines, "→ "+it.Fix)
			return sec
		default:
			if len(p.Items) > 0 {
				sec.Lines = append(sec.Lines, "→ "+p.Items[0].Fix)
			}
		}
		if p.Note != "" {
			sec.Text += " (" + p.Note + ")"
		}
	case "skipped":
		sec.Level, sec.Text = "skipped", "skipped: "+p.Note
	case "error":
		sec.Level, sec.Text = "error", p.Note
	default:
		sec.Level, sec.Text = "skipped", "not checked"
	}
	return sec
}

func dotfilesSection(probes []dotfiles.Probe) Section {
	sec := Section{Name: "dotfiles", Level: "ok"}
	var parts []string
	for _, p := range probes {
		switch {
		case p.Source == "chezmoi" && p.Status == "ok":
			parts = append(parts, "in sync with the repo")
		case p.Source == "chezmoi" && p.Status == "drift":
			parts = append(parts, p.Note)
			sec.Level, sec.Lines = worse(sec.Level, "warn"), append(sec.Lines, "→ "+p.Fix)
		case p.Source == "git" && p.Status == "ok":
			parts = append(parts, "repo in sync with GitHub")
		case p.Source == "git" && p.Status == "drift":
			var g []string
			if n := len(p.Entries); n > 0 {
				g = append(g, fmt.Sprintf("%d uncommitted", n))
			}
			if p.Ahead > 0 {
				g = append(g, fmt.Sprintf("%d to push", p.Ahead))
			}
			if p.Behind > 0 {
				g = append(g, fmt.Sprintf("%d to pull", p.Behind))
			}
			parts = append(parts, strings.Join(g, ", "))
			sec.Level, sec.Lines = worse(sec.Level, "warn"), append(sec.Lines, "→ "+p.Fix)
		case p.Status == "skipped":
			parts = append(parts, p.Source+" skipped: "+p.Note)
			sec.Level = worse(sec.Level, "skipped")
		case p.Status == "error":
			parts = append(parts, p.Source+": "+p.Note)
			sec.Level = "error"
		}
	}
	sec.Text = strings.Join(parts, ", ")
	if sec.Text == "" {
		sec.Level, sec.Text = "skipped", "not checked"
	}
	return sec
}

func appsSection(a Apps) Section {
	var sec Section
	switch {
	case a.Total == 0:
		return Section{Name: "apps", Level: "skipped", Text: "no apps scanned"}
	case len(a.Unknown) == 0:
		sec = Section{Name: "apps", Level: "ok", Text: fmt.Sprintf("%d apps, all with a known updater", a.Total)}
	default:
		sec = Section{Name: "apps", Level: "warn",
			Text:  fmt.Sprintf("%d of %d apps with no known updater: %s", len(a.Unknown), a.Total, strings.Join(first(a.Unknown, 5), ", ")),
			Lines: []string{"→ name them in the overrides file (koizumi apps --help)"}}
	}
	// the overrides travel with the dotfiles, so an entry with no app here is one the other Mac has
	if len(a.NotInstalled) > 0 {
		sec.Lines = append(sec.Lines, "not installed here, but in the overrides: "+strings.Join(first(a.NotInstalled, 3), ", ")+" (koizumi apps)")
	}
	return sec
}

// worse picks the more serious of two levels.
func worse(a, b string) string {
	rank := map[string]int{"ok": 0, "skipped": 1, "warn": 2, "error": 3}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

// Skipped lists the probes that could not run on this machine, so they are visible.
func Skipped(r Report) []Item {
	var items []Item
	for _, p := range r.Outdated {
		if p.Status == "skipped" {
			items = append(items, Item{Level: "skipped", Source: p.Source, Text: p.Note})
		}
	}
	for _, p := range r.Dotfiles {
		if p.Status == "skipped" {
			items = append(items, Item{Level: "skipped", Source: p.Source, Text: p.Note})
		}
	}
	if r.Brewfile.Status == "skipped" {
		items = append(items, Item{Level: "skipped", Source: "Brewfile", Text: r.Brewfile.Note})
	}
	return items
}

// Motd is the sentence a new terminal shows, without the speaker (the command line adds
// "✨ koizumi:"). Empty when nothing needs attention. It counts categories, not items: the
// point is "is there anything?", and the dashboard is one word away. A stale report always
// speaks up, so a dead schedule cannot hide behind silence.
func Motd(r Report, now time.Time) string {
	if now.Sub(r.When) > Stale {
		return fmt.Sprintf("last check was %s; is the schedule alive? koizumi setup", when.Since(r.When, now))
	}
	seen := map[string]bool{}
	n, failed := 0, false
	for _, it := range Attention(r) {
		if !seen[it.Label] {
			seen[it.Label] = true
			n++
		}
		if it.Level == "error" {
			failed = true
		}
	}
	if n == 0 {
		return ""
	}
	s := fmt.Sprintf("%d things are waiting", n)
	if n == 1 {
		s = "1 thing is waiting"
	}
	if failed {
		return s + "; a check failed."
	}
	return s + "."
}

// DefaultPath is where check caches its report: $XDG_STATE_HOME/koizumi/report.json.
func DefaultPath() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "koizumi", "report.json")
}

// Save writes the report atomically (a reader never sees a half-written file).
func Save(r Report, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load reads a cached report.
func Load(path string) (Report, error) {
	var r Report
	data, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	return r, json.Unmarshal(data, &r)
}

func names(items []outdated.Item, max int) string {
	var n []string
	for _, it := range items {
		n = append(n, it.Name)
	}
	return strings.Join(first(n, max), ", ")
}

func first(s []string, max int) []string {
	if len(s) <= max {
		return s
	}
	return append(append([]string{}, s[:max]...), fmt.Sprintf("+%d more", len(s)-max))
}
