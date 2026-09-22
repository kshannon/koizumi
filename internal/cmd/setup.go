package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/kshannon/koizumi/internal/schedule"
	"github.com/spf13/cobra"
)

var setupPrint, setupUninstall bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install the background check on this machine",
	Long: `Writes a launchd agent that runs "koizumi check" at 09:00 and 15:00 and at
login, then loads it. The job runs through your login shell ($SHELL -lc),
so it sees the same environment you do: launchd's own environment has no
Homebrew on its PATH and no XDG_CONFIG_HOME, and without the latter brew
silently leaves out formulae from third-party taps you have trusted.
It logs to ~/Library/Logs/koizumi.log. Safe to run again after koizumi
is reinstalled somewhere else. macOS only for now.

Then add the terminal line to ~/.zshrc:

  command -v koizumi >/dev/null && koizumi motd

--print shows the plist without installing it. --uninstall removes it.`,
	RunE: runSetup,
}

func init() {
	setupCmd.Flags().BoolVar(&setupPrint, "print", false, "print the launchd plist and stop")
	setupCmd.Flags().BoolVar(&setupUninstall, "uninstall", false, "unload and remove the agent")
}

func runSetup(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("setup only knows launchd (macOS) so far; on Linux run \"koizumi check\" from a systemd timer or cron")
	}
	if setupUninstall {
		if err := schedule.Uninstall(); err != nil {
			return err
		}
		fmt.Println("removed the " + schedule.Label + " agent")
		return nil
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(binary); err == nil {
		binary = resolved
	}
	shell := os.Getenv("SHELL")
	if _, err := os.Stat(shell); shell == "" || err != nil {
		shell = "/bin/zsh"
	}
	log, _ := schedule.LogPath()
	if setupPrint {
		fmt.Print(schedule.Plist(shell, binary, log))
		return nil
	}
	path, err := schedule.Install(shell, binary)
	if err != nil {
		return err
	}
	fmt.Printf("installed %s\n  runs:  %s -lc '%s check'   at 09:00, 15:00 and login\n  log:   %s\n", tilde(path), shell, tilde(binary), tilde(log))
	if schedule.Loaded() {
		fmt.Println("  launchd has it " + styleOK.Render("✓"))
	} else {
		fmt.Println("  " + styleWarn.Render("launchd does not report it loaded; check: launchctl print gui/$UID/"+schedule.Label))
	}
	fmt.Println("\nnow add to ~/.zshrc:  command -v koizumi >/dev/null && koizumi motd")
	return nil
}
