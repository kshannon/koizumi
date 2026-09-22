package cmd

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// header renders the first line of a report section as a sentence: a mark, the source, a
// short verdict and a note. ok green ✓, drift/outdated gold ▲, skipped dim ·, error red ✗.
func header(status, source, verdict, note string) string {
	mark := styleOK.Render("✓")
	switch status {
	case "outdated", "drift":
		mark = styleWarn.Render("▲")
	case "skipped":
		mark = styleDim.Render("·")
	case "error":
		mark = styleBad.Render("✗")
	}
	line := mark + " " + styleAccent.Render(source) + ": " + verdict
	if note != "" {
		line += styleDim.Render(", " + note)
	}
	return line
}

// line renders one indented line of a report. A command ("→ ...") is plain text you can
// type, with the arrow in koizumi's colour; anything else is a fact, and dim.
func line(l string) string {
	if cmd, ok := strings.CutPrefix(l, "→ "); ok {
		return styleAccent.Render("→") + " " + cmd
	}
	return styleDim.Render(l)
}

// speak is koizumi saying one sentence: the terminal line, and the one-off notices.
func speak(sentence string) string {
	return "✨ " + styleAccent.Render("koizumi:") + " " + sentence
}

// footnote is an explanation that belongs under a listing, not in it.
func footnote(s string) string { return styleNote.Render(s) }

// signed puts the brigade's mark at the right edge of a header line when there is room.
func signed(head string) string {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return head
	}
	const sig = "SOS団"
	pad := w - lipgloss.Width(head) - lipgloss.Width(sig)
	if pad < 2 {
		return head
	}
	return head + strings.Repeat(" ", pad) + styleAccent.Render(sig)
}

// tilde shortens a path under the home folder to ~/...
func tilde(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
