package apps

import "sort"

// Missing is an override with no matching app on this machine: something another machine
// has, since the overrides file travels with the dotfiles. Note is the comment from the
// file, which for an app installed by hand says where to get it.
type Missing struct {
	Name    string  `json:"name"`
	Updater Updater `json:"updater"`
	Note    string  `json:"note"`
}

// NotInstalled lists the overrides that name no app in installed, sorted by name.
func NotInstalled(overrides map[string]Override, installed []App) []Missing {
	have := map[string]bool{}
	for _, a := range installed {
		have[a.Name] = true
	}
	var out []Missing
	for name, o := range overrides {
		if !have[name] {
			out = append(out, Missing{Name: name, Updater: o.Updater, Note: o.Note})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
