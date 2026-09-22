// Package cmd holds the command-line interface.
package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
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

var jsonOut bool

var root = &cobra.Command{
	Use:   "koizumi",
	Short: "Is my software up to date, and who updates it?",
	Long: `koizumi answers two questions about this machine:

  Is anything out of date?      compared with what each updater offers
  Who updates each app?         the App Store, Homebrew, the app itself, or nobody

It only reports. It never installs or updates anything; it tells you the
command to run instead. With no arguments it shows what needs attention.`,
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runDashboard,
}

func init() {
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "print machine-readable JSON instead of a report")
	root.AddCommand(appsCmd, outdatedCmd, brewCmd, dotfilesCmd, setupCmd)
}

// Execute runs the CLI and exits non-zero on error.
func Execute() {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, styleBad.Render("koizumi: ")+err.Error())
		os.Exit(1)
	}
}

func runDashboard(cmd *cobra.Command, args []string) error {
	fmt.Println(styleTitle.Render("koizumi") + styleDim.Render("  "+Version))
	fmt.Println(styleDim.Render("The dashboard is not built yet. Try: koizumi apps"))
	return nil
}
