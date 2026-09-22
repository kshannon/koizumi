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
  chezmoi apply --less-interactive --verbose
                                    writes the repo's files into ~ and prints
                                    each file it touched; asks before it
                                    replaces any file it did not write itself.
                                    At a prompt the first letter is enough:
                                    d(iff) o(verwrite) a(ll-overwrite) s(kip) q(uit)

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
	Short: "Push the dotfiles; offers one commit for anything uncommitted",
	Long: `git push in the dotfiles repo.

If there are uncommitted changes and you are at a terminal, it shows them and
proposes ONE commit for all of them: a subject naming the areas touched
("Update zshrc, starship") and a body listing every file. Then:

  Enter   commit with that message and push
  e       open the message in $EDITOR first (vim if unset)
  n       stop; nothing is committed

Without a terminal (a script, a cron job) it refuses instead, so nothing is
ever committed unattended. It also refuses to stage a file whose name looks
like a secret (.env, id_*, *.pem, *token*, ...). It says "nothing to push"
when the remote already has everything.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(styleTitle.Render("koizumi push") + styleDim.Render("  ·  "+tilde(dotfilesRepo)))
		return dotsync.Push(dotfilesRepo, os.Stdin, os.Stdout, dotsync.IsTerminal(os.Stdin))
	},
}

func init() {
	for _, c := range []*cobra.Command{pullCmd, pushCmd} {
		c.Flags().StringVar(&dotfilesRepo, "repo", defaultRepo(), "the dotfiles working tree")
	}
}
