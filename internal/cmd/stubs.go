package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

// Commands that are designed but not built yet. Each fails loudly rather than pretending.

var errNotBuilt = errors.New("not built yet")

var outdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "Everything out of date, grouped by who updates it",
	Long: `Compares each installed thing with the latest its updater offers: Homebrew
formulae and casks, App Store apps, macOS itself. For each item it prints the
command (or the click) that updates it. It never runs the update.`,
	RunE: func(*cobra.Command, []string) error { return errNotBuilt },
}

var brewCmd = &cobra.Command{
	Use:   "brew",
	Short: "Brewfile vs what is actually installed",
	Long: `Reads the Brewfile from the dotfiles repo and reports what it lists but is not
installed, and what is installed but not listed.`,
	RunE: func(*cobra.Command, []string) error { return errNotBuilt },
}

var dotfilesCmd = &cobra.Command{
	Use:   "dotfiles",
	Short: "Home folder vs the dotfiles repo",
	Long: `Runs chezmoi status and git status on the dotfiles repo: files in ~ that differ
from the repo, uncommitted changes, commits to push or pull.`,
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
