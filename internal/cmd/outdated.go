package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kshannon/koizumi/internal/outdated"
	"github.com/spf13/cobra"
)

var outdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "Everything out of date, grouped by who updates it",
	Long: `Compares what is installed with the latest each updater offers, and prints
the command to run (or where to click) for each item. Nothing is run.

  Homebrew    brew outdated --json=v2. Formulae and casks that Homebrew
              itself updates. Casks that update themselves are left alone
              (koizumi never uses --greedy). The note says when Homebrew
              last refreshed its index; if that was days ago, run
              "brew update" first or the list is stale.
              To hold a package back: brew pin <name>.
  App Store   mas outdated. Needs mas (brew install mas); skipped otherwise.
  macOS       what Software Update already found, read from its plist in
              no time. (softwareupdate -l would take ~20 s.)

Each source reports its own status: ok, outdated, skipped or error. A
source that could not be checked says so instead of looking fine.`,
	RunE: runOutdated,
}

func runOutdated(cmd *cobra.Command, args []string) error {
	r := outdated.All()
	if jsonOut {
		return json.NewEncoder(os.Stdout).Encode(r)
	}
	printReport(r)
	return nil
}

func printReport(r outdated.Report) {
	pinned := false
	for _, p := range r.Probes {
		verdict := "up to date"
		switch p.Status {
		case "outdated":
			verdict = fmt.Sprintf("%d behind", len(p.Items))
		case "skipped", "error":
			verdict = p.Status
		}
		fmt.Println(header(p.Status, p.Source, verdict, p.Note))
		all := groupFix(p)
		for _, it := range p.Items {
			versions := it.Latest
			if it.Installed != "" {
				versions = it.Installed + " → " + it.Latest
			}
			row := fmt.Sprintf("  %-30s %-26s", it.Name, versions)
			if it.Pinned {
				row += styleDim.Render(" pinned")
				pinned = true
			}
			fmt.Println(row) // the per-item command stays in --json; the group's is enough here
		}
		if all != "" {
			fmt.Println("  " + line("→ "+all))
		}
		fmt.Println()
	}
	if pinned {
		fmt.Println(footnote("pinned: brew upgrade leaves it alone; brew unpin <name> lets it through"))
		fmt.Println()
	}
}

// groupFix is the one command that deals with a whole source at once, or "" if none.
func groupFix(p outdated.Probe) string {
	if p.Status != "outdated" || len(p.Items) == 0 {
		return ""
	}
	switch p.Source {
	case "Homebrew":
		return "brew upgrade"
	case "App Store":
		return "mas upgrade"
	}
	return p.Items[0].Fix
}
