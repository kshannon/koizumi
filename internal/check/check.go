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

// Apps is the short form of the app scan: how many, and which have no known updater.
type Apps struct {
	Total   int      `json:"total"`
	Unknown []string `json:"unknown"`
}

// Options say where to look.
type Options struct {
	Repo      string
	Brewfiles []string
	AppsDir   string
	Overrides string
}

// Run executes every probe now.
func Run(o Options) Report {
	host, _ := os.Hostname()
	r := Report{When: time.Now(), Host: strings.TrimSuffix(host, ".local"), Apps: Apps{Unknown: []string{}}}
	r.Outdated = outdated.All().Probes
	r.Dotfiles = dotfiles.All(o.Repo).Probes
	r.Brewfile = brewfile.Check(o.Brewfiles)
	casks, _ := apps.LoadCasks()
	overrides, _ := apps.LoadOverrides(o.Overrides)
	if list, err := apps.Scan(o.AppsDir, casks, overrides); err == nil {
		r.Apps.Total = len(list)
		for _, a := range list {
			if a.Updater == apps.Unknown {
				r.Apps.Unknown = append(r.Apps.Unknown, a.Name)
			}
		}
	}
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
				it.Text, it.Fix, it.rank = fmt.Sprintf("%d behind", len(p.Items)), "brew upgrade  (koizumi outdated for the list)", 4
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

// Fine lists what is in order, for the dashboard's last line: probes that ran and found
// nothing, in the same order the attention list uses.
func Fine(r Report) []string {
	var fine []string
	for _, p := range r.Outdated {
		if p.Status == "ok" {
			fine = append(fine, p.Source+" current")
		}
	}
	dotfilesOK, dotfilesBad := false, false
	for _, p := range r.Dotfiles {
		switch p.Status {
		case "ok":
			dotfilesOK = true
		case "drift", "error":
			dotfilesBad = true
		}
	}
	if dotfilesOK && !dotfilesBad {
		fine = append(fine, "dotfiles in sync")
	}
	if r.Brewfile.Status == "ok" {
		fine = append(fine, "Brewfile in sync")
	}
	if r.Apps.Total > 0 && len(r.Apps.Unknown) == 0 {
		fine = append(fine, fmt.Sprintf("%d apps, all with a known updater", r.Apps.Total))
	}
	return fine
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

// Motd is the one line a new terminal shows. Empty when nothing needs attention.
// A stale report always speaks up, so a dead schedule cannot hide behind silence.
func Motd(r Report, now time.Time) string {
	if now.Sub(r.When) > Stale {
		return fmt.Sprintf("koizumi · last check %s · is the schedule alive? koizumi setup", when.Since(r.When, now))
	}
	items := Attention(r)
	if len(items) == 0 {
		return ""
	}
	var order []string
	counts := map[string]int{}
	errors := map[string]bool{}
	for _, it := range items {
		if _, seen := counts[it.Label]; !seen {
			order = append(order, it.Label)
		}
		counts[it.Label] += it.Count
		if it.Level == "error" {
			errors[it.Label] = true
		}
	}
	var parts []string
	for _, l := range order {
		if errors[l] {
			parts = append(parts, l+" error")
		} else {
			parts = append(parts, fmt.Sprintf("%s %d", l, counts[l]))
		}
	}
	return "koizumi ▲ " + strings.Join(parts, " · ") + " → koizumi"
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
