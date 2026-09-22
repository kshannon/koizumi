package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kshannon/koizumi/internal/check"
	"github.com/kshannon/koizumi/internal/schedule"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run every check now and cache the result",
	Long: `Runs outdated, dotfiles, brew and apps, saves the combined report to
$XDG_STATE_HOME/koizumi/report.json (default ~/.local/state/koizumi/report.json),
and prints the dashboard. This is what the background schedule runs; a new
terminal reads the cache through "koizumi motd" instead of running anything.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		r := check.Run(options())
		if err := check.Save(r, check.DefaultPath()); err != nil {
			return err
		}
		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(r)
		}
		printDashboard(r, "checked "+r.When.Format("Mon 15:04"))
		return nil
	},
}

var motdCmd = &cobra.Command{
	Use:   "motd",
	Short: "The one line for a new terminal (silent when all is well)",
	Long: `Reads the cached report and prints one line only if something needs attention,
or if the last check is more than a day and a half old (so a dead schedule
cannot hide behind silence). Nothing is run, so it costs nothing at shell
start. Put this in ~/.zshrc:

  command -v koizumi >/dev/null && koizumi motd`,
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := check.Load(check.DefaultPath())
		if os.IsNotExist(err) {
			fmt.Println(styleDim.Render("koizumi · never checked · koizumi setup"))
			return nil
		}
		if err != nil {
			fmt.Println(styleBad.Render("koizumi · cached report is unreadable · koizumi check"))
			return nil
		}
		if line := check.Motd(r, time.Now()); line != "" {
			fmt.Println(styleWarn.Render(line))
		}
		return nil
	},
}

// options collects the same paths the subcommands use, so the dashboard and check agree.
func options() check.Options {
	return check.Options{Repo: dotfilesRepo, Brewfiles: brewfiles, AppsDir: appsDir, Overrides: overridesPath}
}

// printDashboard: a header line, what needs attention, what was skipped, what is fine.
func printDashboard(r check.Report, info string) {
	fmt.Println()
	fmt.Println(styleAccent.Render("koizumi") + styleDim.Render("  ·  "+r.Host+"  ·  "+info))
	items := check.Attention(r)
	for _, it := range items {
		mark := styleWarn.Render("▲")
		if it.Level == "error" {
			mark = styleBad.Render("✗")
		}
		fmt.Printf("%s %-10s %s\n", mark, it.Source, it.Text)
		if it.Fix != "" {
			fmt.Printf("             %s\n", styleDim.Render(it.Fix))
		}
	}
	for _, it := range check.Skipped(r) {
		fmt.Printf("%s %-10s %s\n", styleDim.Render("·"), it.Source, styleDim.Render("skipped: "+it.Text))
	}
	if fine := check.Fine(r); len(fine) > 0 {
		fmt.Println(styleOK.Render("✓") + " " + styleDim.Render(strings.Join(fine, "  ·  ")))
	} else if len(items) == 0 {
		fmt.Println(styleOK.Render("✓") + " nothing needs attention")
	}
	fmt.Println()
}

// nextCheck says when the schedule fires next: "15:00" today or "tomorrow 09:00".
func nextCheck(now time.Time) string {
	n := schedule.Next(now)
	if n.Day() != now.Day() {
		return "tomorrow " + n.Format("15:04")
	}
	return n.Format("15:04")
}
