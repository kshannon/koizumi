package apps

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Override is your answer for an app the scanner cannot work out by itself.
type Override struct {
	Updater Updater
	Note    string // the comment after '#', shown as the reason
}

// DefaultOverridesPath is ~/.config/koizumi/overrides (or $XDG_CONFIG_HOME/koizumi/overrides).
func DefaultOverridesPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = home + "/.config"
	}
	return base + "/koizumi/overrides"
}

// LoadOverrides reads the overrides file. A missing file is fine: no overrides.
func LoadOverrides(path string) (map[string]Override, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return map[string]Override{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	o, err := ParseOverrides(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return o, nil
}

// ParseOverrides reads lines of the form
//
//	App Name = updater   # why
//
// where updater is app-store, brew, self, macos, manual or unknown.
// Blank lines and lines starting with '#' are ignored.
func ParseOverrides(r io.Reader) (map[string]Override, error) {
	out := map[string]Override{}
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		var note string
		if i := strings.Index(line, "#"); i >= 0 {
			line, note = line[:i], strings.TrimSpace(line[i+1:])
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if !ok || name == "" {
			return nil, fmt.Errorf("line %d: expected \"App Name = updater\", got %q", n, sc.Text())
		}
		u := Updater(value)
		if !validUpdaters[u] {
			return nil, fmt.Errorf("line %d: %q is not an updater (app-store, brew, self, macos, manual, unknown)", n, value)
		}
		out[name] = Override{Updater: u, Note: note}
	}
	return out, sc.Err()
}
