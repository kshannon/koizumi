package cmd

import (
	"os"
	"strings"
)

// header renders the first line of a report section: a coloured mark, the source, a short
// verdict and a dim note. status colours: ok green, drift/outdated yellow, skipped dim, error red.
func header(status, source, verdict, note string) string {
	mark := styleOK.Render("■")
	switch status {
	case "outdated", "drift":
		mark = styleWarn.Render("■")
	case "skipped":
		mark = styleDim.Render("■")
	case "error":
		mark = styleBad.Render("■")
	}
	line := mark + " " + styleTitle.Render(source) + "  " + verdict
	if note != "" {
		line += styleDim.Render("  · " + note)
	}
	return line
}

// tilde shortens a path under the home folder to ~/...
func tilde(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
