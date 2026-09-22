package outdated

import (
	"context"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"
)

const repo = "https://github.com/kshannon/koizumi"

// Self asks whether this koizumi binary is behind the repo's main branch. A binary from
// `go install ...@main` carries a pseudo-version ending in the commit sha; that is compared
// with GitHub's current main. A build from source ("(devel)") cannot be compared.
func Self() Probe {
	p := Probe{Source: "koizumi"}
	installed := "(devel)"
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		installed = bi.Main.Version
	}
	if strings.Contains(installed, "devel") {
		return skipped(p, "built from source, not compared")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "ls-remote", repo, "refs/heads/main").Output()
	if err != nil {
		return skipped(p, "could not reach GitHub to compare")
	}
	latest := strings.Fields(string(out))
	if len(latest) == 0 {
		return skipped(p, "could not read GitHub's main")
	}
	behind, why := SelfBehind(installed, latest[0])
	if why != "" {
		return skipped(p, why)
	}
	p.Note = "installed " + short(installed)
	if !behind {
		return done(p, nil)
	}
	return done(p, []Item{{Source: "koizumi", Name: "koizumi", Installed: short(installed), Latest: latest[0][:7],
		Fix: "GOPROXY=direct go install github.com/kshannon/koizumi@main"}})
}

// SelfBehind compares an installed module version with main's commit sha. It only knows
// pseudo-versions (untagged installs); a tagged release is not compared to main.
func SelfBehind(installed, latestSHA string) (behind bool, why string) {
	if installed == "" || strings.Contains(installed, "devel") {
		return false, "built from source, not compared"
	}
	i := strings.LastIndex(installed, "-")
	if i < 0 || len(installed)-i-1 != 12 {
		return false, "" // a tagged release: compared by tag once releases exist
	}
	sha := installed[i+1:]
	return !strings.HasPrefix(latestSHA, sha), ""
}

func short(v string) string {
	if i := strings.LastIndex(v, "-"); i >= 0 && len(v)-i-1 == 12 {
		return v[i+1 : i+8]
	}
	return v
}
