package cmd

import (
	"encoding/json"
	"fmt"
	"os"

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

--brewfile can be given more than once. The default is
$DOTFILES/brew/Brewfile.common, else ~/dev/dotfiles/brew/Brewfile.common.`,
	RunE: runBrew,
}

func init() {
	def := os.Getenv("DOTFILES")
	if def == "" {
		home, _ := os.UserHomeDir()
		def = home + "/dev/dotfiles"
	}
	brewCmd.Flags().StringSliceVar(&brewfiles, "brewfile", []string{def + "/brew/Brewfile.common"}, "Brewfile to check (repeatable)")
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
	if p.Status != "skipped" && p.Status != "error" {
		note = tilde(p.Files[0]) + " · " + p.Note
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
	if p.Fix != "" {
		fmt.Println("  " + styleDim.Render(p.Fix))
	}
	fmt.Println()
	return nil
}
