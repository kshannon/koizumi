package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/kshannon/koizumi/internal/brewfile"
	"github.com/spf13/cobra"
)

var brewfiles []string

var brewCmd = &cobra.Command{
	Use:   "brew",
	Short: "Brewfile vs what is actually installed",
	Long: `Reads the Brewfile(s) and compares them with Homebrew, both ways:

  missing   listed in the Brewfile but not installed
  extra     installed but not listed

Formulae are compared against what was installed on request, so a library
that only came in as a dependency is never reported as extra. Taps and
casks are compared too. App Store (mas) entries are not checked.

Nothing is installed or removed. The fix for missing entries is
"brew bundle --no-upgrade --file=<Brewfile>"; extras are yours to add to
the Brewfile or uninstall.

--brewfile can be given more than once. The default is Brewfile.common plus
this machine's own Brewfile.<hostname>, when that file exists, under
$DOTFILES/brew else ~/dev/dotfiles/brew.`,
	RunE: runBrew,
}

func init() {
	brewCmd.Flags().StringSliceVar(&brewfiles, "brewfile", defaultBrewfiles(), "Brewfile to check (repeatable)")
}

// defaultBrewfiles is Brewfile.common, plus brew/Brewfile.<hostname> when this machine has one.
// Naming the per-machine file after the hostname keeps the choice out of the code: a new Mac is
// covered the day you add brew/Brewfile.<its hostname>, with nothing to edit or release. A file
// that is not there is left out rather than passed on: brewfile.Check reports a missing file as
// an error, and most machines will only ever have the shared one.
func defaultBrewfiles() []string {
	def := os.Getenv("DOTFILES")
	if def == "" {
		home, _ := os.UserHomeDir()
		def = home + "/dev/dotfiles"
	}
	files := []string{def + "/brew/Brewfile.common"}
	host, err := os.Hostname()
	if err != nil {
		return files
	}
	// trimmed the same way check.go does, so the dashboard header and this cannot disagree
	// about which machine we are on
	mine := def + "/brew/Brewfile." + strings.TrimSuffix(host, ".local")
	if _, err := os.Stat(mine); err == nil {
		files = append(files, mine)
	}
	return files
}

func runBrew(cmd *cobra.Command, args []string) error {
	p := brewfile.Check(brewfiles)
	if jsonOut {
		return json.NewEncoder(os.Stdout).Encode(p)
	}
	verdict := "in sync"
	switch p.Status {
	case "drift":
		verdict = "drift"
	case "skipped", "error":
		verdict = p.Status
	}
	note := p.Note
	if p.Status != "skipped" && p.Status != "error" && len(p.Files) > 0 {
		// name every file read, so it is visible whether this machine's own Brewfile was picked
		// up; the first in full, the rest by name since they sit beside it
		names := []string{tilde(p.Files[0])}
		for _, f := range p.Files[1:] {
			names = append(names, path.Base(f))
		}
		note = strings.Join(names, " + ") + ", " + p.Note
	}
	fmt.Println(header(p.Status, p.Source, verdict, note))
	rows := []struct {
		what, kind string
		names      []string
	}{
		{"missing", "tap", p.Diff.MissingTaps}, {"missing", "formula", p.Diff.MissingFormulae}, {"missing", "cask", p.Diff.MissingCasks},
		{"extra", "tap", p.Diff.ExtraTaps}, {"extra", "formula", p.Diff.ExtraFormulae}, {"extra", "cask", p.Diff.ExtraCasks},
	}
	for _, r := range rows {
		for _, n := range r.names {
			fmt.Printf("  %-8s %-8s %s\n", r.what, r.kind, n)
		}
	}
	for _, l := range strings.Split(p.Fix, "\n") {
		if l != "" {
			fmt.Println("  " + line(l))
		}
	}
	fmt.Println()
	return nil
}
