package outdated

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// mainCommit is GitHub's view of the repo's main branch: its commit sha and when it was made.
const mainCommit = "https://api.github.com/repos/kshannon/koizumi/commits/main"

// Running says what this binary is, for --version: the tag, else the commit and its time,
// else that it was built from source.
func Running() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		return describe(bi.Main.Version)
	}
	return describe("")
}

func describe(installed string) string {
	if installed == "" || strings.Contains(installed, "devel") {
		return "dev, built from source"
	}
	if sha, _ := pseudoSHA(installed); sha == "" {
		return installed // a tag
	}
	s := short(installed)
	if at, ok := pseudoTime(installed); ok {
		s += " from " + stamp(at)
	}
	return s
}

// Self asks whether this koizumi binary is behind the repo's main branch. A binary from
// `go install ...@main` carries a pseudo-version ending in the commit's time and sha; that
// is compared with GitHub's current main. A build from source ("(devel)") cannot be compared.
func Self() Probe {
	installed := "(devel)"
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		installed = bi.Main.Version
	}
	if strings.Contains(installed, "devel") {
		return skipped(Probe{Source: "koizumi"}, "built from source, not compared")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, mainCommit, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return skipped(Probe{Source: "koizumi"}, "could not reach GitHub to compare (running "+short(installed)+")")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	sha, at, err := ParseGitHubCommit(body)
	if err != nil {
		return skipped(Probe{Source: "koizumi"}, "could not read GitHub's main: "+err.Error()+" (running "+short(installed)+")")
	}
	return selfProbe(installed, sha, at)
}

// selfProbe builds the probe from what is running and what main is. It always says which
// commit is running and when it was made; behind, the item carries both sides.
func selfProbe(installed, mainSHA string, mainAt time.Time) Probe {
	p := Probe{Source: "koizumi"}
	behind, why := SelfBehind(installed, mainSHA)
	if why != "" {
		return skipped(p, why)
	}
	running := short(installed)
	if at, ok := pseudoTime(installed); ok {
		running += " from " + stamp(at)
	}
	p.Note = "running " + running
	if !behind {
		return done(p, nil)
	}
	return done(p, []Item{{Source: "koizumi", Name: "koizumi", Installed: running,
		Latest: mainSHA[:7] + " from " + stamp(mainAt),
		Fix:    "go install github.com/kshannon/koizumi@main"}})
}

// ParseGitHubCommit reads GitHub's commit JSON: the sha and the committer date.
func ParseGitHubCommit(body []byte) (sha string, at time.Time, err error) {
	var raw struct {
		SHA    string `json:"sha"`
		Commit struct {
			Committer struct {
				Date time.Time `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", time.Time{}, err
	}
	if len(raw.SHA) < 7 {
		if raw.Message != "" {
			return "", time.Time{}, fmt.Errorf("%s", raw.Message)
		}
		return "", time.Time{}, fmt.Errorf("no commit in the reply")
	}
	return raw.SHA, raw.Commit.Committer.Date, nil
}

// SelfBehind compares an installed module version with main's commit sha. It only knows
// pseudo-versions (untagged installs); a tagged release is not compared to main.
func SelfBehind(installed, latestSHA string) (behind bool, why string) {
	if installed == "" || strings.Contains(installed, "devel") {
		return false, "built from source, not compared"
	}
	sha, _ := pseudoSHA(installed)
	if sha == "" {
		return false, "" // a tagged release: compared by tag once releases exist
	}
	return !strings.HasPrefix(latestSHA, sha), ""
}

// pseudoSHA is the 12-char commit sha a pseudo-version ends in, and whether the binary was
// built from a modified checkout (`go install .` stamps "+dirty" after the sha). "" for a tag.
func pseudoSHA(v string) (sha string, dirty bool) {
	v, dirty = strings.CutSuffix(v, "+dirty")
	i := strings.LastIndex(v, "-")
	if i < 0 || len(v)-i-1 != 12 {
		return "", false
	}
	return v[i+1:], dirty
}

// pseudoTime is the commit time a pseudo-version carries (vX.Y.Z-yyyymmddhhmmss-sha, UTC).
func pseudoTime(v string) (time.Time, bool) {
	v, _ = strings.CutSuffix(v, "+dirty")
	parts := strings.Split(v, "-")
	if len(parts) < 3 {
		return time.Time{}, false
	}
	t, err := time.Parse("20060102150405", parts[len(parts)-2])
	return t, err == nil
}

// short is the 7-char sha for display, with a trailing "+" when the checkout was modified.
func short(v string) string {
	sha, dirty := pseudoSHA(v)
	if sha == "" {
		return v
	}
	if dirty {
		return sha[:7] + "+"
	}
	return sha[:7]
}

// stamp prints a commit time in local time, to the minute.
func stamp(t time.Time) string { return t.Local().Format("2006-01-02 15:04") }
