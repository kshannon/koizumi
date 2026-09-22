package dotsync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kshannon/koizumi/internal/dotfiles"
)

func TestPushPlan(t *testing.T) {
	tests := []struct {
		name string
		p    dotfiles.Probe
		run  bool
		why  string
	}{
		{"uncommitted changes block a push", dotfiles.Probe{Branch: "main", Upstream: true, Ahead: 1, Entries: []dotfiles.Entry{{Status: "M", Path: "x"}}}, false, "1 uncommitted change(s): commit first"},
		{"no upstream", dotfiles.Probe{Branch: "chezmoi"}, false, "no upstream branch: git push -u origin chezmoi"},
		{"nothing ahead", dotfiles.Probe{Branch: "main", Upstream: true}, false, "nothing to push: main is up to date with its upstream"},
		{"ahead pushes", dotfiles.Probe{Branch: "main", Upstream: true, Ahead: 2}, true, "pushing 2 commit(s) on main"},
		{"a failed probe never pushes", dotfiles.Probe{Status: "error", Note: "not a git repository"}, false, "not a git repository"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run, why := PushPlan(tt.p)
			if run != tt.run || why != tt.why {
				t.Fatalf("got run=%v why=%q, want run=%v why=%q", run, why, tt.run, tt.why)
			}
		})
	}
}

func TestApplyPlan(t *testing.T) {
	if run, why := ApplyPlan(dotfiles.Probe{Status: "skipped", Note: "not set up"}); run || why != "not set up" {
		t.Errorf("skipped chezmoi must not apply: run=%v why=%q", run, why)
	}
	if run, _ := ApplyPlan(dotfiles.Probe{Status: "ok"}); !run {
		t.Error("a set-up chezmoi should apply")
	}
	if run, _ := ApplyPlan(dotfiles.Probe{Status: "drift"}); !run {
		t.Error("drift should apply")
	}
}

// GitPull fast-forwards a real clone from a real origin.
func TestGitPullFastForwards(t *testing.T) {
	base := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	origin := filepath.Join(base, "origin.git")
	git(base, "init", "-q", "--bare", "-b", "main", origin)
	a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
	git(base, "clone", "-q", origin, a)
	os.WriteFile(filepath.Join(a, "f"), []byte("1"), 0o644)
	git(a, "add", "f")
	git(a, "commit", "-q", "-m", "one")
	git(a, "push", "-q", "-u", "origin", "main")
	git(base, "clone", "-q", origin, b)
	os.WriteFile(filepath.Join(a, "f"), []byte("2"), 0o644)
	git(a, "commit", "-q", "-am", "two")
	git(a, "push", "-q")

	if err := GitPull(b); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(b, "f")); string(got) != "2" {
		t.Fatalf("b was not fast-forwarded, f = %q", got)
	}
}

func TestDraftBuildsOneCommitForEverything(t *testing.T) {
	entries := []dotfiles.Entry{
		{Status: "M", Path: "home/dot_zshrc"},
		{Status: "M", Path: "home/dot_config/starship.toml"},
		{Status: "??", Path: "home/dot_config/koizumi/overrides"},
		{Status: "M", Path: "brew/Brewfile.common"},
		{Status: "M", Path: "docs/howto/tmux.md"},
		{Status: "M", Path: "home/private_dot_ssh/config"},
	}
	subject, body := Draft(entries)
	if want := "Update zshrc, starship, koizumi, Brewfile +2 more"; subject != want {
		t.Errorf("subject\n got  %q\n want %q", subject, want)
	}
	for _, line := range []string{"M home/dot_zshrc", "A home/dot_config/koizumi/overrides", "M home/private_dot_ssh/config"} { // ?? becomes A
		if !strings.Contains(body, line) {
			t.Errorf("body is missing %q:\n%s", line, body)
		}
	}
	if strings.Contains(body, "---") {
		t.Error("no --- line: git stops reading trailers at one")
	}
	if s, _ := Draft([]dotfiles.Entry{{Status: "M", Path: "home/dot_zshrc"}}); s != "Update zshrc" {
		t.Errorf("single area: %q", s)
	}
	// a bare directory entry ("home/dot_config/") has no file to name: skipped, no trailing comma
	if s, _ := Draft([]dotfiles.Entry{{Status: "M", Path: "home/dot_zshrc"}, {Status: "??", Path: "home/dot_config/"}}); s != "Update zshrc" {
		t.Errorf("directory entry: %q", s)
	}
}

func TestIsTerminalIsFalseForDevNullAndPipes(t *testing.T) {
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	if IsTerminal(devnull) {
		t.Error("/dev/null is a character device but not a terminal")
	}
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()
	if IsTerminal(r) {
		t.Error("a pipe is not a terminal")
	}
}

func TestSecretish(t *testing.T) {
	for _, p := range []string{"home/private_dot_ssh/id_ed25519", ".env", "home/dot_env.local", "secrets/x.pem", "creds/token.json", "home/dot_netrc"} {
		if !Secretish(p) {
			t.Errorf("%q should look secret", p)
		}
	}
	for _, p := range []string{"home/dot_zshrc", "home/private_dot_ssh/config", "docs/howto/tmux.md", "home/dot_config/koizumi/overrides"} {
		if Secretish(p) {
			t.Errorf("%q should not look secret", p)
		}
	}
}
