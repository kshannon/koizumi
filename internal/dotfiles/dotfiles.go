// Package dotfiles checks whether this machine still matches the dotfiles repo:
// the files in ~ against what chezmoi would write, and the repo against its remote.
package dotfiles

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kshannon/koizumi/internal/when"
)

// Entry is one file that differs, with the tool's own status code (M, A, D, MM, ...).
type Entry struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

// Probe is the result of one check.
type Probe struct {
	Source  string  `json:"source"` // chezmoi, git
	Status  string  `json:"status"` // ok, drift, skipped, error
	Note    string  `json:"note,omitempty"`
	Fix     string  `json:"fix,omitempty"`
	Entries []Entry `json:"entries"`

	// git only
	Repo     string `json:"repo,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Upstream bool   `json:"upstream,omitempty"`
	Ahead    int    `json:"ahead,omitempty"`
	Behind   int    `json:"behind,omitempty"`
}

// Report is both probes, run now.
type Report struct {
	When   time.Time `json:"when"`
	Probes []Probe   `json:"probes"`
}

// All runs both probes. repo is the dotfiles working tree.
func All(repo string) Report {
	return Report{When: time.Now(), Probes: []Probe{Chezmoi(repo), Git(repo)}}
}

// Chezmoi asks chezmoi which files in ~ differ from what the repo says.
func Chezmoi(repo string) Probe {
	p := Probe{Source: "chezmoi", Entries: []Entry{}}
	chezmoi, err := exec.LookPath("chezmoi")
	if err != nil {
		return skipped(p, "chezmoi is not installed")
	}
	src, err := exec.Command(chezmoi, "source-path").Output()
	if err != nil {
		return failed(p, "chezmoi source-path", err)
	}
	if _, err := os.Stat(strings.TrimSpace(string(src))); err != nil {
		return skipped(p, "not set up on this machine: chezmoi init --source "+repo)
	}
	out, err := exec.Command(chezmoi, "status").Output()
	if err != nil {
		return failed(p, "chezmoi status", err)
	}
	p.Entries = ParseStatus(string(out))
	if p.Entries == nil {
		p.Entries = []Entry{}
	}
	p.Status = "ok"
	if n := len(p.Entries); n > 0 {
		p.Status = "drift"
		p.Note = fmt.Sprintf("%d file(s) in ~ differ from the repo", n)
		p.Fix = "chezmoi diff, then chezmoi apply --less-interactive · to keep a change made in ~: chezmoi re-add ~/<file>"
	}
	return p
}

// ParseStatus reads `chezmoi status` or `git status --porcelain`: a two-column code, a space, a path.
func ParseStatus(out string) []Entry {
	var entries []Entry
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		entries = append(entries, Entry{Status: strings.TrimSpace(line[:2]), Path: strings.TrimSpace(line[3:])})
	}
	return entries
}

// Git reports the repo's own state: uncommitted files, commits to push or pull. It fetches
// first (a fetch touches no files of yours, only what git knows about the remote), so
// "behind" is real; if the fetch fails, "behind" is as of the last fetch and the note says so.
func Git(repo string) Probe {
	p := Probe{Source: "git", Repo: repo, Entries: []Entry{}}
	// Only the trailing newline is trimmed: porcelain lines start with a meaningful space.
	git := func(args ...string) (string, error) {
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
		return strings.TrimRight(string(out), "\n"), err
	}
	branch, err := git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return failed(p, repo, fmt.Errorf("not a git repository"))
	}
	p.Branch = branch
	fetched := true
	{
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "git", "-C", repo, "fetch", "--quiet").Run(); err != nil {
			fetched = false
		}
	}
	// --untracked-files=all lists each new file; the default collapses a new directory into one line.
	if out, err := git("status", "--porcelain", "--untracked-files=all"); err == nil && out != "" {
		p.Entries = ParseStatus(out + "\n")
	}
	notes := []string{branch}
	if out, err := git("rev-list", "--left-right", "--count", "@{u}...HEAD"); err == nil {
		if behind, ahead, err := ParseAheadBehind(out); err == nil {
			p.Upstream, p.Behind, p.Ahead = true, behind, ahead
			notes = append(notes, fmt.Sprintf("ahead %d", ahead), fmt.Sprintf("behind %d", behind))
		}
	} else {
		notes = append(notes, "no upstream")
	}
	if !fetched {
		notes = append(notes, "fetch failed")
	}
	if t, ok := lastFetch(repo); ok {
		notes = append(notes, "fetched "+ago(t))
	}
	p.Note = strings.Join(notes, " · ")

	var fixes []string
	if len(p.Entries) > 0 {
		fixes = append(fixes, "commit in "+repo)
	}
	if p.Ahead > 0 {
		fixes = append(fixes, "git push")
	}
	if p.Behind > 0 {
		fixes = append(fixes, "git pull --ff-only, then chezmoi apply")
	}
	p.Status = "ok"
	if len(fixes) > 0 {
		p.Status, p.Fix = "drift", strings.Join(fixes, " · ")
	}
	return p
}

// ParseAheadBehind reads `git rev-list --left-right --count @{u}...HEAD`: "<behind>\t<ahead>".
func ParseAheadBehind(out string) (behind, ahead int, err error) {
	f := strings.Fields(out)
	if len(f) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output %q", out)
	}
	if behind, err = strconv.Atoi(f[0]); err != nil {
		return 0, 0, err
	}
	if ahead, err = strconv.Atoi(f[1]); err != nil {
		return 0, 0, err
	}
	return behind, ahead, nil
}

// lastFetch is the mtime of FETCH_HEAD, looked up in the worktree's git dir then the common one.
func lastFetch(repo string) (time.Time, bool) {
	for _, flag := range []string{"--git-dir", "--git-common-dir"} {
		out, err := exec.Command("git", "-C", repo, "rev-parse", flag).Output()
		if err != nil {
			continue
		}
		dir := strings.TrimSpace(string(out))
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(repo, dir)
		}
		if fi, err := os.Stat(filepath.Join(dir, "FETCH_HEAD")); err == nil {
			return fi.ModTime(), true
		}
	}
	return time.Time{}, false
}

func skipped(p Probe, why string) Probe { p.Status, p.Note = "skipped", why; return p }

func failed(p Probe, what string, err error) Probe {
	msg := err.Error()
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		msg = strings.TrimSpace(string(ee.Stderr))
	}
	p.Status, p.Note = "error", what+": "+msg
	return p
}

func ago(t time.Time) string { return when.Ago(t) }
