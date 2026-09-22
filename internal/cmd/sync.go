package cmd

import (
	"fmt"
	"os"

	"github.com/kshannon/koizumi/internal/dotsync"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull the dotfiles repo and apply it to ~",
	Long: `Two steps, in order:

  git pull --ff-only                in the dotfiles repo
  chezmoi apply --less-interactive  writes the repo's files into ~, asking
                                    before it replaces any file it did not
                                    write itself

The apply step is skipped, and says so, when chezmoi is not set up on this
machine. Nothing is committed and no software is updated.

The repo is --repo, else $DOTFILES, else ~/dev/dotfiles.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(styleTitle.Render("koizumi pull") + styleDim.Render("  ·  "+tilde(dotfilesRepo)))
		if err := dotsync.Pull(dotfilesRepo, os.Stdout); err != nil {
			return err
		}
		fmt.Println(styleOK.Render("✓") + " done")
		return nil
	},
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push committed dotfiles to the remote",
	Long: `git push in the dotfiles repo. It refuses with a clear message when there are
uncommitted changes (commit first; a commit is yours to write) or no upstream
branch, and says "nothing to push" when the remote already has everything.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(styleTitle.Render("koizumi push") + styleDim.Render("  ·  "+tilde(dotfilesRepo)))
		return dotsync.Push(dotfilesRepo, os.Stdout)
	},
}

func init() {
	for _, c := range []*cobra.Command{pullCmd, pushCmd} {
		c.Flags().StringVar(&dotfilesRepo, "repo", defaultRepo(), "the dotfiles working tree")
	}
}
