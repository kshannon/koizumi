package cmd

import (
	"encoding/json"
	"fmt"
	"os"
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
		printDashboard(r, "checked just now, next "+nextCheck(r.When))
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
			fmt.Println(speak("never checked; run koizumi setup."))
			return nil
		}
		if err != nil {
			fmt.Println(speak(styleBad.Render("the cached report is unreadable; run koizumi check.")))
			return nil
		}
		if s := check.Motd(r, time.Now()); s != "" {
			fmt.Println(speak(s))
		}
		return nil
	},
}

// options collects the same paths the subcommands use, so the dashboard and check agree.
func options() check.Options {
	return check.Options{Repo: dotfilesRepo, Brewfiles: brewfiles, AppsDir: appsDir, Overrides: overridesPath}
}

// printDashboard: one header sentence, then every category with its mark; a ✓ is one line,
// a ▲ or ✗ earns the command under it. When nothing needs you, it says so and stops.
func printDashboard(r check.Report, info string) {
	fmt.Println()
	fmt.Println(signed("✨ " + styleAccent.Render("koizumi") + " on " + r.Host + ", " + info))
	fine := true
	for _, s := range check.Sections(r) {
		fmt.Println()
		mark, text := styleOK.Render("✓"), s.Text
		switch s.Level {
		case "warn":
			mark, fine = styleWarn.Render("▲"), false
		case "error":
			mark, fine = styleBad.Render("✗"), false
		case "skipped":
			mark, text = styleDim.Render("·"), styleDim.Render(s.Text)
		}
		fmt.Printf("%s %-10s %s\n", mark, s.Name, text)
		for _, l := range s.Lines {
			fmt.Println("             " + line(l))
		}
	}
	fmt.Println()
	if fine {
		fmt.Println(styleDim.Render("nothing to report. as expected."))
		fmt.Println()
	}
}

// nextCheck says when the schedule fires next: "at 15:00" today or "tomorrow at 09:00".
func nextCheck(now time.Time) string {
	n := schedule.Next(now)
	if n.Day() != now.Day() {
		return "tomorrow at " + n.Format("15:04")
	}
	return "at " + n.Format("15:04")
}
