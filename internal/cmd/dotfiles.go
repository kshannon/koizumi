package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kshannon/koizumi/internal/dotfiles"
	"github.com/spf13/cobra"
)

var dotfilesRepo string

var dotfilesCmd = &cobra.Command{
	Use:   "dotfiles",
	Short: "Home folder vs the dotfiles repo",
	Long: `Two checks, nothing changed:

  chezmoi   chezmoi status: files in ~ that differ from what the repo would
            write (a tool edited one, or the repo moved on). Skipped when
            chezmoi is not installed or not set up on this machine.
  git       the repo itself: uncommitted files, commits to push, commits to
            pull. It never fetches, so "behind" is as of the last fetch;
            the note says when that was.

The repo is --repo, else $DOTFILES, else ~/dev/dotfiles.`,
	RunE: runDotfiles,
}

func init() {
	def := os.Getenv("DOTFILES")
	if def == "" {
		home, _ := os.UserHomeDir()
		def = home + "/dev/dotfiles"
	}
	dotfilesCmd.Flags().StringVar(&dotfilesRepo, "repo", def, "the dotfiles working tree")
}

func runDotfiles(cmd *cobra.Command, args []string) error {
	r := dotfiles.All(dotfilesRepo)
	if jsonOut {
		return json.NewEncoder(os.Stdout).Encode(r)
	}
	for _, p := range r.Probes {
		head := "in sync"
		switch p.Status {
		case "drift":
			head = "drift"
		case "skipped", "error":
			head = p.Status
		}
		note := p.Note
		if p.Source == "git" && p.Status != "error" {
			note = tilde(p.Repo) + " · " + p.Note
		}
		fmt.Println(header(p.Status, p.Source, head, note))
		for _, e := range p.Entries {
			fmt.Printf("  %-3s %s\n", e.Status, e.Path)
		}
		if p.Fix != "" {
			fmt.Println("  " + styleDim.Render(p.Fix))
		}
		fmt.Println()
	}
	return nil
}
