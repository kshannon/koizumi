// Package brewfile compares a Brewfile with what Homebrew actually has installed.
// It never installs or removes anything.
package brewfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Entry is one line of a Brewfile: tap, brew, cask or mas, and its name.
type Entry struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

var entryLine = regexp.MustCompile(`^\s*(tap|brew|cask|mas)\s+"([^"]+)"`)

// Parse reads a Brewfile. Options after the name (restart_service, id, args) are ignored.
func Parse(r io.Reader) []Entry {
	var entries []Entry
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if m := entryLine.FindStringSubmatch(sc.Text()); m != nil {
			entries = append(entries, Entry{Kind: m[1], Name: m[2]})
		}
	}
	return entries
}

// Installed is what Homebrew reports: formulae installed on request (not as dependencies),
// casks, and taps.
type Installed struct {
	Formulae []string
	Casks    []string
	Taps     []string
}

// Diff is the two-way comparison. Missing = in the Brewfile, not installed.
// Extra = installed, not in the Brewfile.
type Diff struct {
	MissingFormulae []string `json:"missing_formulae"`
	ExtraFormulae   []string `json:"extra_formulae"`
	MissingCasks    []string `json:"missing_casks"`
	ExtraCasks      []string `json:"extra_casks"`
	MissingTaps     []string `json:"missing_taps"`
	ExtraTaps       []string `json:"extra_taps"`
}

// Any says whether anything differs at all.
func (d Diff) Any() bool {
	return len(d.MissingFormulae)+len(d.ExtraFormulae)+len(d.MissingCasks)+len(d.ExtraCasks)+len(d.MissingTaps)+len(d.ExtraTaps) > 0
}

// Compare matches Brewfile entries against what is installed. Formulae and casks match on
// the last path segment, so `brew "heroku/brew/heroku"` matches the installed `heroku`.
// mas entries are not compared.
func Compare(entries []Entry, inst Installed) Diff {
	var wantFormulae, wantCasks, wantTaps []string
	for _, e := range entries {
		switch e.Kind {
		case "brew":
			wantFormulae = append(wantFormulae, e.Name)
		case "cask":
			wantCasks = append(wantCasks, e.Name)
		case "tap":
			wantTaps = append(wantTaps, e.Name)
		}
	}
	var d Diff
	d.MissingFormulae, d.ExtraFormulae = diff(wantFormulae, inst.Formulae, path.Base)
	d.MissingCasks, d.ExtraCasks = diff(wantCasks, inst.Casks, path.Base)
	d.MissingTaps, d.ExtraTaps = diff(wantTaps, inst.Taps, func(s string) string { return s })
	return d
}

// diff returns want-not-have and have-not-want, comparing through key, sorted, never nil.
func diff(want, have []string, key func(string) string) (missing, extra []string) {
	missing, extra = []string{}, []string{}
	haveKeys := map[string]bool{}
	for _, h := range have {
		haveKeys[key(h)] = true
	}
	wantKeys := map[string]bool{}
	for _, w := range want {
		wantKeys[key(w)] = true
		if !haveKeys[key(w)] {
			missing = append(missing, w)
		}
	}
	for _, h := range have {
		if !wantKeys[key(h)] {
			extra = append(extra, h)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}

// Probe is the result of checking one or more Brewfiles against this machine.
type Probe struct {
	Source string   `json:"source"` // Brewfile
	Status string   `json:"status"` // ok, drift, skipped, error
	Note   string   `json:"note,omitempty"`
	Fix    string   `json:"fix,omitempty"`
	Files  []string `json:"files"`
	Diff   Diff     `json:"diff"`
}

// Check reads the Brewfiles, asks Homebrew what is installed, and compares.
func Check(files []string) Probe {
	p := Probe{Source: "Brewfile", Files: files}
	brew, err := exec.LookPath("brew")
	if err != nil {
		p.Status, p.Note = "skipped", "Homebrew is not installed"
		return p
	}
	var entries []Entry
	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			p.Status, p.Note = "error", err.Error()
			return p
		}
		entries = append(entries, Parse(fh)...)
		fh.Close()
	}
	inst, err := installed(brew)
	if err != nil {
		p.Status, p.Note = "error", err.Error()
		return p
	}
	p.Diff = Compare(entries, inst)
	counts := map[string]int{}
	for _, e := range entries {
		counts[e.Kind]++
	}
	p.Note = fmt.Sprintf("%s, %s, %s listed", plural(counts["brew"], "formula", "formulae"),
		plural(counts["cask"], "cask", "casks"), plural(counts["tap"], "tap", "taps"))
	if counts["mas"] > 0 {
		p.Note += fmt.Sprintf(", %s not checked", plural(counts["mas"], "App Store entry", "App Store entries"))
	}
	p.Status = "ok"
	if p.Diff.Any() {
		p.Status = "drift"
		var fixes []string
		if len(p.Diff.MissingFormulae)+len(p.Diff.MissingCasks)+len(p.Diff.MissingTaps) > 0 {
			for _, f := range files {
				fixes = append(fixes, "install what is missing: brew bundle --no-upgrade --file="+f)
			}
		}
		if len(p.Diff.ExtraFormulae)+len(p.Diff.ExtraCasks)+len(p.Diff.ExtraTaps) > 0 {
			fixes = append(fixes, "extras: add them to the Brewfile, or brew uninstall <name>; koizumi never removes anything")
		}
		p.Fix = strings.Join(fixes, " · ")
	}
	return p
}

// installed asks Homebrew three questions. Formulae are those installed on request,
// which is what `brew bundle dump` would write.
func installed(brew string) (Installed, error) {
	run := func(args ...string) ([]string, error) {
		cmd := exec.Command(brew, args...)
		cmd.Env = append(cmd.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1")
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("brew %s: %w", strings.Join(args, " "), err)
		}
		var names []string
		for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				names = append(names, l)
			}
		}
		return names, nil
	}
	var inst Installed
	var err error
	if inst.Formulae, err = run("list", "--formula", "--installed-on-request", "-1"); err != nil {
		return inst, err
	}
	if inst.Casks, err = run("list", "--cask", "-1"); err != nil {
		return inst, err
	}
	if inst.Taps, err = run("tap"); err != nil {
		return inst, err
	}
	return inst, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
