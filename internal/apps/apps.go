// Package apps answers one question for every app in /Applications:
// who updates it?
package apps

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"howett.net/plist"
)

// Updater is who keeps an app current.
type Updater string

const (
	AppStore Updater = "app-store" // the Mac App Store
	Brew     Updater = "brew"      // Homebrew, when you run `brew upgrade`
	Self     Updater = "self"      // the app has its own updater
	Unknown  Updater = "unknown"   // nothing found: check by hand
)

// Facts are the things the scanner can observe about one app bundle.
type Facts struct {
	MASReceipt       bool   // Contents/_MASReceipt/receipt exists
	SparkleFeed      bool   // Info.plist has SUFeedURL (Sparkle) or KSUpdateURL (Google Keystone)
	UpdaterFramework bool   // bundles Sparkle, Squirrel or Keystone
	Cask             string // Homebrew cask token, "" if not installed by brew
	CaskAutoUpdates  bool   // Homebrew says the cask updates itself
}

// Classify decides who updates an app and says why. An override always wins.
func Classify(f Facts, override Updater) (Updater, string) {
	switch {
	case override != "":
		return override, "override"
	case f.MASReceipt:
		return AppStore, "App Store receipt"
	case f.SparkleFeed:
		return Self, "update feed in Info.plist"
	case f.UpdaterFramework:
		return Self, "bundled updater"
	case f.CaskAutoUpdates:
		return Self, "cask marked auto_updates"
	case f.Cask != "":
		return Brew, "Homebrew cask, no self-updater"
	default:
		return Unknown, "no updater found"
	}
}

// App is one entry in /Applications.
type App struct {
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	Version string  `json:"version"`
	Cask    string  `json:"cask,omitempty"` // set when Homebrew installed it
	Updater Updater `json:"updater"`
	Reason  string  `json:"reason"`
}

// Cask is what Homebrew knows about an installed cask.
type Cask struct {
	Token       string
	AutoUpdates bool
	Apps        []string // app bundle names it installs, e.g. "Raycast.app"
}

// Scan reads every .app under dir and works out who updates it.
// casks is keyed by app bundle name; overrides by app name (without .app).
func Scan(dir string, casks map[string]Cask, overrides map[string]Updater) ([]App, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var apps []App
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".app") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		name := strings.TrimSuffix(e.Name(), ".app")
		info := readInfoPlist(filepath.Join(path, "Contents", "Info.plist"))
		f := Facts{
			MASReceipt:       exists(filepath.Join(path, "Contents", "_MASReceipt", "receipt")),
			SparkleFeed:      info["SUFeedURL"] != "" || info["KSUpdateURL"] != "",
			UpdaterFramework: hasUpdaterFramework(filepath.Join(path, "Contents")),
		}
		if c, ok := casks[e.Name()]; ok {
			f.Cask, f.CaskAutoUpdates = c.Token, c.AutoUpdates
		}
		updater, reason := Classify(f, overrides[name])
		apps = append(apps, App{Name: name, Path: path, Version: info["CFBundleShortVersionString"],
			Cask: f.Cask, Updater: updater, Reason: reason})
	}
	sort.Slice(apps, func(i, j int) bool { return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name) })
	return apps, nil
}

// readInfoPlist returns the string values of an Info.plist (XML or binary); missing file = empty map.
func readInfoPlist(path string) map[string]string {
	out := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var raw map[string]any
	if _, err := plist.Unmarshal(data, &raw); err != nil {
		return out
	}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// isUpdaterBundle says whether a bundle name means "this app updates itself": the Sparkle,
// Squirrel (Electron) and Keystone (Google) frameworks, or any helper app/bundle with
// "updater" or "autoupdate" in its name (Mozilla's updater.app, ZoomAutoUpdater.app, ...).
func isUpdaterBundle(name string) bool {
	switch name {
	case "Sparkle.framework", "Squirrel.framework", "KeystoneRegistration.framework":
		return true
	}
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".app") && !strings.HasSuffix(lower, ".bundle") {
		return false
	}
	return strings.Contains(lower, "updater") || strings.Contains(lower, "autoupdate")
}

// hasUpdaterFramework walks Contents/ a few levels deep (Chrome nests Keystone inside a
// versioned framework) looking for any known updater bundle.
func hasUpdaterFramework(contents string) bool {
	const maxDepth = 6
	found := false
	filepath.WalkDir(contents, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return filepath.SkipDir
		}
		if isUpdaterBundle(d.Name()) {
			found = true
			return filepath.SkipAll
		}
		if d.IsDir() && strings.Count(strings.TrimPrefix(path, contents), string(os.PathSeparator)) >= maxDepth {
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
