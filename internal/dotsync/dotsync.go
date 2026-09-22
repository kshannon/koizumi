// Package dotsync moves the dotfiles between this machine and the repo: pull (then apply)
// and push. push can commit, but only at a terminal, only after showing you the message,
// and never anything that looks like a secret.
package dotsync

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kshannon/koizumi/internal/dotfiles"
	"golang.org/x/term"
)

// Pull fast-forwards the repo, then applies it with chezmoi (which asks before it
// overwrites any file it did not write). Output goes to out; git and chezmoi keep the
// terminal, so chezmoi's prompts work.
func Pull(repo string, out io.Writer) error {
	fmt.Fprintln(out, "→ git pull --ff-only")
	if err := GitPull(repo); err != nil {
		return fmt.Errorf("git pull failed; fix the repo state in %s and run again", repo)
	}
	if run, why := ApplyPlan(dotfiles.Chezmoi(repo)); !run {
		fmt.Fprintf(out, "· chezmoi apply skipped: %s\n", why)
		return nil
	}
	fmt.Fprintln(out, "→ chezmoi apply --less-interactive --verbose")
	if err := passthrough("chezmoi", "apply", "--less-interactive", "--verbose"); err != nil {
		return fmt.Errorf("chezmoi apply stopped: %w", err)
	}
	return nil
}

// Push pushes committed work. With uncommitted changes and a terminal, it offers to make
// ONE commit for everything (a drafted message you can accept or edit), then pushes.
// Without a terminal it refuses, so nothing is ever committed unattended.
func Push(repo string, in io.Reader, out io.Writer, interactive bool) error {
	p := dotfiles.Git(repo)
	if p.Status == "error" {
		return fmt.Errorf("%s", p.Note)
	}
	if len(p.Entries) > 0 {
		if !interactive {
			return fmt.Errorf("%d uncommitted change(s): run koizumi push in a terminal to commit them, or commit by hand", len(p.Entries))
		}
		for _, e := range p.Entries {
			if Secretish(e.Path) {
				return fmt.Errorf("refusing: %s looks like a secret; move it out of the repo or add it to .gitignore", e.Path)
			}
		}
		committed, err := offerCommit(repo, p.Entries, in, out)
		if err != nil || !committed {
			return err
		}
		p = dotfiles.Git(repo)
	}
	run, why := PushPlan(p)
	if !run && isRefusal(why) {
		return fmt.Errorf("%s", why) // printed once, by the error handler
	}
	fmt.Fprintln(out, "→ "+why)
	if !run {
		return nil
	}
	return passthrough("git", "-C", repo, "push")
}

// offerCommit shows the changes and a drafted message, then asks:
// Enter commits, e opens $EDITOR on the draft, anything else stops.
func offerCommit(repo string, entries []dotfiles.Entry, in io.Reader, out io.Writer) (bool, error) {
	subject, body := Draft(entries)
	fmt.Fprintf(out, "%d uncommitted change(s):\n", len(entries))
	for _, e := range entries {
		fmt.Fprintf(out, "  %-2s %s\n", e.Status, e.Path)
	}
	fmt.Fprintf(out, "\nproposed commit:\n  %s\n\n", subject)
	fmt.Fprint(out, "[Enter] commit and push · [e] edit the message · [n] stop: ")
	answer, _ := bufio.NewReader(in).ReadString('\n')
	message := subject + "\n\n" + body + "\n"
	switch strings.TrimSpace(strings.ToLower(answer)) {
	case "", "y", "yes":
	case "e", "edit":
		edited, err := editMessage(message)
		if err != nil {
			return false, err
		}
		if strings.TrimSpace(edited) == "" {
			fmt.Fprintln(out, "empty message, nothing committed")
			return false, nil
		}
		message = edited
	default:
		fmt.Fprintln(out, "stopped, nothing committed")
		return false, nil
	}
	return true, Commit(repo, message)
}

// Draft builds one commit message for a set of changed files: a subject naming the areas
// touched, and a body listing every file. No "---" line: git stops reading trailers there.
func Draft(entries []dotfiles.Entry) (subject, body string) {
	var areas []string
	seen := map[string]bool{}
	var lines []string
	for _, e := range entries {
		if a := area(e.Path); a != "" && !seen[a] {
			seen[a] = true
			areas = append(areas, a)
		}
		status := e.Status
		if status == "??" {
			status = "A" // untracked now, added by the commit
		}
		lines = append(lines, status+" "+e.Path)
	}
	const max = 4
	if len(areas) > max {
		areas = append(areas[:max:max], fmt.Sprintf("+%d more", len(areas)-max))
		subject = "Update " + strings.Join(areas[:max], ", ") + " " + areas[max]
	} else {
		subject = "Update " + strings.Join(areas, ", ")
	}
	return subject, strings.Join(lines, "\n")
}

var attrPrefixes = []string{"private_", "executable_", "readonly_", "empty_", "modify_", "create_", "symlink_", "exact_", "dot_"}

// area names what a path is about: home/dot_zshrc -> zshrc, home/dot_config/starship.toml
// -> starship, brew/Brewfile.common -> Brewfile, docs/howto/tmux.md -> docs.
func area(path string) string {
	segs := strings.Split(path, "/")
	if segs[0] == "home" && len(segs) > 1 {
		segs = segs[1:]
		for i := range segs {
			segs[i] = stripAttrs(segs[i])
		}
		if segs[0] == "config" && len(segs) > 1 {
			return stripExt(segs[1])
		}
		return stripExt(segs[0])
	}
	switch segs[0] {
	case "brew":
		return "Brewfile"
	case "docs", "scripts":
		return segs[0]
	}
	return stripExt(segs[len(segs)-1])
}

func stripAttrs(seg string) string {
	for changed := true; changed; {
		changed = false
		for _, p := range attrPrefixes {
			if strings.HasPrefix(seg, p) {
				seg, changed = strings.TrimPrefix(seg, p), true
			}
		}
	}
	return seg
}

func stripExt(name string) string {
	name = strings.TrimSuffix(name, ".tmpl")
	if i := strings.LastIndex(name, "."); i > 0 {
		name = name[:i]
	}
	return strings.TrimPrefix(name, ".")
}

// Secretish says whether a path looks like it holds a secret, by name only.
func Secretish(path string) bool {
	lower := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	base = "." + stripAttrs(strings.TrimPrefix(base, "."))
	base = strings.TrimPrefix(base, "..")
	base = strings.Replace(base, ".dot_", ".", 1)
	if strings.HasPrefix(base, ".id_") && !strings.HasSuffix(base, ".pub") {
		return true
	}
	for _, s := range []string{".env", ".netrc", "auth.json", "credentials", "hosts.yml"} {
		if strings.HasPrefix(strings.TrimPrefix(base, "."), strings.TrimPrefix(s, ".")) {
			return true
		}
	}
	for _, ext := range []string{".pem", ".key", ".p12", ".pfx"} {
		if strings.HasSuffix(base, ext) {
			return true
		}
	}
	for _, word := range []string{"secret", "credential", "token", "password", "passwd"} {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

// Commit stages everything in the repo and commits it with message.
func Commit(repo, message string) error {
	if err := passthrough("git", "-C", repo, "add", "-A"); err != nil {
		return err
	}
	f, err := os.CreateTemp("", "koizumi-commit-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(message); err != nil {
		return err
	}
	f.Close()
	return passthrough("git", "-C", repo, "commit", "-q", "-F", f.Name())
}

// editMessage opens the draft in $EDITOR (vim if unset) and returns what was saved.
func editMessage(draft string) (string, error) {
	f, err := os.CreateTemp("", "koizumi-commit-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(draft); err != nil {
		return "", err
	}
	f.Close()
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	if err := passthrough("sh", "-c", editor+" \"$1\"", "koizumi", f.Name()); err != nil {
		return "", fmt.Errorf("editor: %w", err)
	}
	out, err := os.ReadFile(f.Name())
	return string(out), err
}

// PushPlan decides what push should do from the repo's git state.
func PushPlan(p dotfiles.Probe) (run bool, why string) {
	switch {
	case p.Status == "error":
		return false, p.Note
	case len(p.Entries) > 0:
		return false, fmt.Sprintf("%d uncommitted change(s): commit first", len(p.Entries))
	case !p.Upstream:
		return false, "no upstream branch: git push -u origin " + p.Branch
	case p.Ahead == 0:
		return false, fmt.Sprintf("nothing to push: %s is up to date with its upstream", p.Branch)
	default:
		return true, fmt.Sprintf("pushing %d commit(s) on %s", p.Ahead, p.Branch)
	}
}

// ApplyPlan decides whether chezmoi apply should run after a pull: only when chezmoi is
// set up on this machine.
func ApplyPlan(p dotfiles.Probe) (run bool, why string) {
	if p.Status == "skipped" || p.Status == "error" {
		return false, p.Note
	}
	return true, ""
}

// GitPull runs `git pull --ff-only` in repo with the terminal attached.
func GitPull(repo string) error {
	return passthrough("git", "-C", repo, "pull", "--ff-only")
}

// isRefusal tells a "do not push" that is an error (exit 1) from "nothing to push" (exit 0).
func isRefusal(why string) bool {
	return !strings.HasPrefix(why, "nothing to push")
}

// IsTerminal reports whether f is an interactive terminal. A character-device check is not
// enough: /dev/null is one too, and a push reading Enter from it would commit unattended.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

func passthrough(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
