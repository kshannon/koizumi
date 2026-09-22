// Package outdated compares what is installed with the latest each updater offers.
// Every probe reports its own status, so a probe that could not run never reads as "all fine".
package outdated

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/kshannon/koizumi/internal/when"
	"howett.net/plist"
)

// Item is one thing that is behind.
type Item struct {
	Source    string `json:"source"` // brew, cask, app-store, macos
	Name      string `json:"name"`
	Installed string `json:"installed"`
	Latest    string `json:"latest"`
	Pinned    bool   `json:"pinned,omitempty"`
	Fix       string `json:"fix"` // the command to run, or where to click
}

// Probe is the result of asking one updater.
type Probe struct {
	Source string `json:"source"` // Homebrew, App Store, macOS
	Status string `json:"status"` // ok, outdated, skipped, error
	Note   string `json:"note,omitempty"`
	Items  []Item `json:"items"`
}

// Report is every probe, run now.
type Report struct {
	When   time.Time `json:"when"`
	Probes []Probe   `json:"probes"`
}

// All runs every probe.
func All() Report {
	return Report{When: time.Now(), Probes: []Probe{Brew(), AppStore(), MacOS(), Self()}}
}

// Brew asks Homebrew what is behind. Self-updating casks are left to themselves (no --greedy).
func Brew() Probe {
	p := Probe{Source: "Homebrew"}
	brew, err := exec.LookPath("brew")
	if err != nil {
		return skipped(p, "Homebrew is not installed")
	}
	cmd := exec.Command(brew, "outdated", "--json=v2")
	cmd.Env = append(cmd.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1")
	out, err := cmd.Output()
	if err != nil {
		return failed(p, "brew outdated", err)
	}
	items, err := ParseBrewOutdated(out)
	if err != nil {
		return failed(p, "reading brew's JSON", err)
	}
	p.Note = brewIndexAge(brew)
	return done(p, items)
}

// ParseBrewOutdated reads `brew outdated --json=v2`.
func ParseBrewOutdated(data []byte) ([]Item, error) {
	var payload struct {
		Formulae []brewEntry `json:"formulae"`
		Casks    []brewEntry `json:"casks"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	var items []Item
	for _, f := range payload.Formulae {
		it := Item{Source: "brew", Name: f.Name, Installed: strings.Join(f.Installed, ", "), Latest: f.Current, Pinned: f.Pinned}
		it.Fix = "brew upgrade " + f.Name
		if f.Pinned {
			it.Fix = fmt.Sprintf("pinned: brew unpin %s, then brew upgrade %s", f.Name, f.Name)
		}
		items = append(items, it)
	}
	for _, c := range payload.Casks {
		items = append(items, Item{Source: "cask", Name: c.Name, Installed: strings.Join(c.Installed, ", "), Latest: c.Current,
			Fix: "brew upgrade --cask " + c.Name})
	}
	return items, nil
}

type brewEntry struct {
	Name      string   `json:"name"`
	Installed []string `json:"installed_versions"`
	Current   string   `json:"current_version"`
	Pinned    bool     `json:"pinned"`
}

// brewIndexAge says when Homebrew last fetched its package index (brew update).
func brewIndexAge(brew string) string {
	repo, err := exec.Command(brew, "--repository").Output()
	if err != nil {
		return ""
	}
	fi, err := os.Stat(filepath.Join(strings.TrimSpace(string(repo)), ".git", "FETCH_HEAD"))
	if err != nil {
		return "index age unknown"
	}
	return "index refreshed " + ago(fi.ModTime()) + " (brew update)"
}

// AppStore asks mas, the App Store command-line tool, if it is installed.
func AppStore() Probe {
	p := Probe{Source: "App Store"}
	mas, err := exec.LookPath("mas")
	if err != nil {
		return skipped(p, "install mas to check: brew install mas")
	}
	out, err := exec.Command(mas, "outdated").Output()
	if err != nil {
		return failed(p, "mas outdated", err)
	}
	return done(p, ParseMasOutdated(string(out)))
}

var masLine = regexp.MustCompile(`^\s*(\d+)\s+(.+?) \((.+?) -> (.+?)\)\s*$`)

// ParseMasOutdated reads lines like "497799835 Xcode (15.0 -> 15.1)".
func ParseMasOutdated(out string) []Item {
	var items []Item
	for _, line := range strings.Split(out, "\n") {
		m := masLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		items = append(items, Item{Source: "app-store", Name: strings.TrimSpace(m[2]), Installed: m[3], Latest: m[4], Fix: "mas upgrade " + m[1]})
	}
	return items
}

const softwareUpdatePlist = "/Library/Preferences/com.apple.SoftwareUpdate.plist"

// MacOS reads what Software Update already found. Reading its plist takes no time;
// `softwareupdate -l` would take twenty seconds and talk to Apple.
func MacOS() Probe {
	p := Probe{Source: "macOS"}
	if runtime.GOOS != "darwin" {
		return skipped(p, "not a Mac")
	}
	data, err := os.ReadFile(softwareUpdatePlist)
	if err != nil {
		return failed(p, "reading "+softwareUpdatePlist, err)
	}
	ver, _ := exec.Command("sw_vers", "-productVersion").Output()
	items, checked, err := ParseSoftwareUpdate(data, strings.TrimSpace(string(ver)))
	if err != nil {
		return failed(p, "reading "+softwareUpdatePlist, err)
	}
	if checked.IsZero() {
		p.Note = "never checked"
	} else {
		p.Note = "last checked " + ago(checked)
	}
	return done(p, items)
}

// ParseSoftwareUpdate reads com.apple.SoftwareUpdate.plist. installed is the running macOS version.
func ParseSoftwareUpdate(data []byte, installed string) ([]Item, time.Time, error) {
	var raw struct {
		LastSuccessfulDate time.Time `plist:"LastSuccessfulDate"`
		Recommended        []struct {
			Name    string `plist:"Display Name"`
			Version string `plist:"Display Version"`
		} `plist:"RecommendedUpdates"`
	}
	if _, err := plist.Unmarshal(data, &raw); err != nil {
		return nil, time.Time{}, err
	}
	var items []Item
	for _, u := range raw.Recommended {
		it := Item{Source: "macos", Name: u.Name, Latest: u.Version, Fix: "System Settings > General > Software Update"}
		if strings.HasPrefix(u.Name, "macOS") {
			it.Installed = installed
		}
		items = append(items, it)
	}
	return items, raw.LastSuccessfulDate, nil
}

// The three ways a probe ends. Items is always a real slice, never nil, so the JSON
// says "items": [] rather than null.
func done(p Probe, items []Item) Probe {
	p.Items = items
	if p.Items == nil {
		p.Items = []Item{}
	}
	p.Status = "ok"
	if len(items) > 0 {
		p.Status = "outdated"
	}
	return p
}

func skipped(p Probe, why string) Probe {
	p.Status, p.Note, p.Items = "skipped", why, []Item{}
	return p
}

func failed(p Probe, what string, err error) Probe {
	msg := err.Error()
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		msg = strings.TrimSpace(string(ee.Stderr))
	}
	p.Status, p.Note, p.Items = "error", what+": "+msg, []Item{}
	return p
}

func ago(t time.Time) string { return when.Ago(t) }
