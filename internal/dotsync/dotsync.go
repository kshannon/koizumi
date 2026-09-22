// Package dotsync moves the dotfiles between this machine and the repo: pull (then apply)
// and push. It never commits: a commit is a decision with a message, and that is yours.
package dotsync

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/kshannon/koizumi/internal/dotfiles"
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
	fmt.Fprintln(out, "→ chezmoi apply --less-interactive")
	if err := passthrough("chezmoi", "apply", "--less-interactive"); err != nil {
		return fmt.Errorf("chezmoi apply stopped: %w", err)
	}
	return nil
}

// Push pushes committed work. It refuses when there are uncommitted changes or no upstream.
func Push(repo string, out io.Writer) error {
	run, why := PushPlan(dotfiles.Git(repo))
	if !run && isRefusal(why) {
		return fmt.Errorf("%s", why) // printed once, by the error handler
	}
	fmt.Fprintln(out, "→ "+why)
	if !run {
		return nil
	}
	return passthrough("git", "-C", repo, "push")
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

func passthrough(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
