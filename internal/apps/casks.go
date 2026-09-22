package apps

import (
	"encoding/json"
	"os/exec"
)

// LoadCasks asks Homebrew about every installed cask. Without brew it returns an empty map,
// so a machine with no Homebrew still gets a scan (everything then reads as hand-installed).
func LoadCasks() (map[string]Cask, error) {
	brew, err := exec.LookPath("brew")
	if err != nil {
		return map[string]Cask{}, nil
	}
	cmd := exec.Command(brew, "info", "--cask", "--json=v2", "--installed")
	cmd.Env = append(cmd.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return ParseCasks(out)
}

// ParseCasks turns `brew info --cask --json=v2` output into a map keyed by app bundle name.
func ParseCasks(data []byte) (map[string]Cask, error) {
	var payload struct {
		Casks []struct {
			Token       string           `json:"token"`
			AutoUpdates *bool            `json:"auto_updates"` // null when brew doesn't know
			Artifacts   []map[string]any `json:"artifacts"`
		} `json:"casks"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	byApp := map[string]Cask{}
	for _, c := range payload.Casks {
		cask := Cask{Token: c.Token, AutoUpdates: c.AutoUpdates != nil && *c.AutoUpdates}
		for _, art := range c.Artifacts {
			apps, _ := art["app"].([]any)
			for _, a := range apps {
				if name, ok := a.(string); ok {
					cask.Apps = append(cask.Apps, name)
					byApp[name] = cask
				}
			}
		}
	}
	return byApp, nil
}
