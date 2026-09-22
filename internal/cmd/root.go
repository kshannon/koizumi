// Package cmd holds the command-line interface.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/kshannon/koizumi/internal/check"
	"github.com/spf13/cobra"
)

// Version is set at build time by goreleaser (-X ...cmd.Version=v0.1.0).
var Version = "dev"

var (
	styleTitle = lipgloss.NewStyle().Bold(true)
	styleDim   = lipgloss.NewStyle().Faint(true)
	styleWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	styleOK    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	styleBad   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red
)

var jsonOut, cached bool

var root = &cobra.Command{
	Use:   "koizumi",
	Short: "Is my software up to date, and who updates it?",
	Long: `koizumi answers three questions about this machine:

  Is anything out of date?          compared with what each updater offers
  Who updates each app?             the App Store, Homebrew, the app itself, or nobody
  Does it still match my dotfiles?  Brewfile vs installed, home folder vs the repo

It only reports. It never installs, updates or changes anything; where
something needs doing it prints the command to run. With no arguments it
shows only what needs attention.

Each command's --help says exactly what it checks and how it decides.`,
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runDashboard,
}

func init() {
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "print machine-readable JSON instead of a report")
	root.Flags().BoolVar(&cached, "cached", false, "show the last check instead of running one")
	root.AddCommand(appsCmd, outdatedCmd, brewCmd, dotfilesCmd, checkCmd, motdCmd, setupCmd)
}

// Execute runs the CLI and exits non-zero on error.
func Execute() {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, styleBad.Render("koizumi: ")+err.Error())
		os.Exit(1)
	}
}

// runDashboard shows only what needs attention. Live by default (a few seconds, mostly
// Homebrew); --cached shows the last background check instead.
func runDashboard(cmd *cobra.Command, args []string) error {
	var r check.Report
	if cached {
		var err error
		if r, err = check.Load(check.DefaultPath()); err != nil {
			return fmt.Errorf("no cached report yet: run koizumi check (or koizumi setup)")
		}
	} else {
		r = check.Run(options())
		_ = check.Save(r, check.DefaultPath()) // keep the terminal line current; not fatal
	}
	if jsonOut {
		return json.NewEncoder(os.Stdout).Encode(r)
	}
	printDashboard(r)
	return nil
}
