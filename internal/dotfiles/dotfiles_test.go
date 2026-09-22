package dotfiles

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseChezmoiStatus(t *testing.T) {
	out := " M .zshrc\n A .gitignore_global\nMM .config/starship.toml\n"
	got := ParseStatus(out)
	want := []Entry{{"M", ".zshrc"}, {"A", ".gitignore_global"}, {"MM", ".config/starship.toml"}}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if ParseStatus("") != nil {
		t.Error("empty output should give no entries")
	}
}

func TestParseAheadBehind(t *testing.T) {
	behind, ahead, err := ParseAheadBehind("2\t5\n") // git rev-list --left-right --count @{u}...HEAD
	if err != nil || behind != 2 || ahead != 5 {
		t.Fatalf("got behind=%d ahead=%d err=%v, want 2 5", behind, ahead, err)
	}
	if _, _, err := ParseAheadBehind("garbage"); err == nil {
		t.Error("garbage should be an error")
	}
}

// Git runs against a real repository: one commit, one uncommitted change, no upstream.
func TestGitProbeOnRealRepo(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	run("add", "a.txt")
	run("commit", "-q", "-m", "first")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644)
	// an untracked file inside a NEW directory must be listed by file, not as "newdir/"
	os.MkdirAll(filepath.Join(dir, "newdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "newdir", "x.txt"), []byte("x"), 0o644)

	p := Git(dir)
	if p.Status != "drift" {
		t.Fatalf("status = %q (%s), want drift", p.Status, p.Note)
	}
	var paths []string
	for _, e := range p.Entries {
		paths = append(paths, e.Path)
	}
	if len(paths) != 2 || paths[0] != "a.txt" || paths[1] != "newdir/x.txt" {
		t.Errorf("entries = %v, want [a.txt newdir/x.txt]", paths)
	}
	if p.Branch != "main" {
		t.Errorf("branch = %q, want main", p.Branch)
	}
	if p.Upstream {
		t.Error("a fresh repo has no upstream")
	}

	notARepo := Git(t.TempDir())
	if notARepo.Status != "error" {
		t.Errorf("a folder that is not a repo should be an error, got %q", notARepo.Status)
	}
}
