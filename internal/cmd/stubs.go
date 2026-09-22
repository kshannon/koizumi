package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

// Commands that are designed but not built yet. Each fails loudly rather than pretending.

var errNotBuilt = errors.New("not built yet")

var brewCmd = &cobra.Command{
	Use:   "brew",
	Short: "Brewfile vs what is actually installed",
	Long: `Reads the Brewfile from the dotfiles repo and reports what it lists but is not
installed, and what is installed but not listed.`,
	RunE: func(*cobra.Command, []string) error { return errNotBuilt },
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install the background check on this machine",
	Long: `Writes a launchd agent (macOS) or systemd timer (Linux) that runs the checks
twice a day and caches the result, so a new terminal can show a one-line
summary without running anything slow.`,
	RunE: func(*cobra.Command, []string) error { return errNotBuilt },
}
