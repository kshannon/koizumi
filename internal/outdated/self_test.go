package outdated

import (
	"strings"
	"testing"
	"time"
)

func TestSelfCompare(t *testing.T) {
	// go install ...@main stamps a pseudo-version ending in the 12-char commit sha
	inst := "v0.0.0-20260922070445-3f9f81691dfc"
	if behind, why := SelfBehind(inst, "3f9f81691dfc0a1b2c3d4e5f60718293a4b5c6d7"); behind || why != "" {
		t.Errorf("same commit must not be behind: %v %q", behind, why)
	}
	if behind, _ := SelfBehind(inst, "473f944aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); !behind {
		t.Error("a different commit on main means behind")
	}
	if behind, why := SelfBehind("(devel)", "473f944aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); behind || why == "" {
		t.Errorf("a source build cannot be compared: want not behind with a reason, got %v %q", behind, why)
	}
	if behind, _ := SelfBehind("v0.1.0", "473f944aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); behind {
		t.Error("a tagged release is compared by tag later, not by main's sha")
	}
	// `go install .` from a modified checkout stamps "+dirty" after the sha (Yuki's binary
	// read as "current" because that suffix hid the sha from the comparison)
	dirty := "v0.0.0-20260922155716-473f94443904+dirty"
	if behind, why := SelfBehind(dirty, "700c1c1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); !behind || why != "" {
		t.Errorf("dirty build of an older commit must be behind: got %v %q", behind, why)
	}
	if behind, _ := SelfBehind(dirty, "473f94443904aaaaaaaaaaaaaaaaaaaaaaaaaaaa"); behind {
		t.Error("dirty build of main's own commit is not behind")
	}
	if got := short(dirty); got != "473f944+" {
		t.Errorf("short(dirty) = %q, want the sha marked as modified", got)
	}
}

// The pseudo-version also carries the commit time (UTC), so koizumi can say when the
// build it is running came through, without asking anyone.
func TestPseudoTime(t *testing.T) {
	got, ok := pseudoTime("v0.0.0-20260922155716-473f94443904+dirty")
	want := time.Date(2026, 9, 22, 15, 57, 16, 0, time.UTC)
	if !ok || !got.Equal(want) {
		t.Errorf("got %v %v, want %v true", got, ok, want)
	}
	if _, ok := pseudoTime("v0.1.0"); ok {
		t.Error("a tag carries no commit time")
	}
}

func TestParseGitHubCommit(t *testing.T) {
	body := `{"sha":"700c1c1db962aaaaaaaaaaaaaaaaaaaaaaaaaaaa","commit":{"committer":{"date":"2026-09-22T16:10:49Z"}}}`
	sha, at, err := ParseGitHubCommit([]byte(body))
	if err != nil || sha != "700c1c1db962aaaaaaaaaaaaaaaaaaaaaaaaaaaa" || !at.Equal(time.Date(2026, 9, 22, 16, 10, 49, 0, time.UTC)) {
		t.Errorf("got %q %v %v", sha, at, err)
	}
	if _, _, err := ParseGitHubCommit([]byte(`{"message":"Not Found"}`)); err == nil {
		t.Error("a reply without a sha must be an error, never 'current'")
	}
}

// The probe always says which commit is running and when it was made; behind, it also
// says what GitHub's main is and when.
func TestSelfProbe(t *testing.T) {
	older := "v0.0.0-20260922155716-473f94443904"
	newer := "v0.0.0-20260922161049-700c1c1db962"
	mainSHA, mainAt := "700c1c1db962aaaaaaaaaaaaaaaaaaaaaaaaaaaa", time.Date(2026, 9, 22, 16, 10, 49, 0, time.UTC)

	p := selfProbe(newer, mainSHA, mainAt)
	if p.Status != "ok" || len(p.Items) != 0 {
		t.Fatalf("same commit: status %q items %d", p.Status, len(p.Items))
	}
	if want := "running 700c1c1 from " + stamp(mainAt); p.Note != want {
		t.Errorf("note %q, want %q", p.Note, want)
	}

	p = selfProbe(older, mainSHA, mainAt)
	if p.Status != "outdated" || len(p.Items) != 1 {
		t.Fatalf("older commit: status %q items %d", p.Status, len(p.Items))
	}
	it := p.Items[0]
	if want := "473f944 from " + stamp(time.Date(2026, 9, 22, 15, 57, 16, 0, time.UTC)); it.Installed != want {
		t.Errorf("installed %q, want %q", it.Installed, want)
	}
	if want := "700c1c1 from " + stamp(mainAt); it.Latest != want {
		t.Errorf("latest %q, want %q", it.Latest, want)
	}
	if !strings.Contains(it.Fix, "go install github.com/kshannon/koizumi@main") {
		t.Errorf("fix %q", it.Fix)
	}
	if p.Note != "running "+it.Installed {
		t.Errorf("behind, the note still says what is running: %q", p.Note)
	}

	if p := selfProbe("(devel)", mainSHA, mainAt); p.Status != "skipped" {
		t.Errorf("source build: status %q", p.Status)
	}
}

// --version says what the binary knows about itself: the tag, else the commit and its
// time from the pseudo-version, else that it is a source build.
func TestDescribe(t *testing.T) {
	at := stamp(time.Date(2026, 9, 22, 18, 24, 38, 0, time.UTC))
	for in, want := range map[string]string{
		"v0.0.0-20260922182438-ed88e64abcde":       "ed88e64 from " + at,
		"v0.0.0-20260922182438-ed88e64abcde+dirty": "ed88e64+ from " + at,
		"v0.1.0":  "v0.1.0",
		"(devel)": "dev, built from source",
		"":        "dev, built from source",
	} {
		if got := describe(in); got != want {
			t.Errorf("describe(%q) = %q, want %q", in, got, want)
		}
	}
}
