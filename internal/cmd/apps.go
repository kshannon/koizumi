package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kshannon/koizumi/internal/apps"
	"github.com/spf13/cobra"
)

var appsDir, overridesPath string

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "Every app in /Applications and who updates it",
	Long: `Lists every app in /Applications (or --dir) and who keeps it current:

  app-store   the Mac App Store updates it
  self        the app updates itself; use its own "check for updates"
  brew        Homebrew installed it and nothing else updates it: brew upgrade
  macos       comes with macOS: Software Update (from the overrides file)
  manual      you download it by hand (from the overrides file)
  unknown     no updater found: check by hand

How it decides, in order. The first match wins:

  0. the app is named in the overrides file                  -> that answer

  1. Contents/_MASReceipt/receipt exists                    -> app-store
  2. Info.plist has SUFeedURL (Sparkle) or KSUpdateURL      -> self
     (Google Keystone)
  3. the bundle contains Sparkle, Squirrel or Keystone       -> self
     frameworks, or an updater helper app (updater.app,
     *AutoUpdater.app, ...), up to six folders deep
  4. Homebrew's cask list marks it auto_updates              -> self
  5. Homebrew installed it, with no updater found            -> brew
  6. none of the above                                       -> unknown

"brew cask" after a version means Homebrew installed the app, whoever
updates it: a cask marked auto_updates is upgraded by the app itself, and
plain brew upgrade leaves it alone (brew upgrade --greedy would not).
Homebrew is asked with: brew info --cask --json=v2 --installed.

Some apps update themselves but leave no trace in the bundle (Microsoft
AutoUpdate, Steam). Tell koizumi about those in the overrides file,
~/.config/koizumi/overrides, one app per line:

  Microsoft Teams = self     # Microsoft AutoUpdate
  Steam           = self     # updates itself on launch
  Safari          = macos    # comes with macOS
  Anki            = manual   # download from the website

The text after # is shown as the reason. Values: app-store, brew, self,
macos, manual, unknown.`,
	RunE: runApps,
}

func init() {
	appsCmd.Flags().StringVar(&appsDir, "dir", "/Applications", "folder to scan")
	appsCmd.Flags().StringVar(&overridesPath, "overrides", apps.DefaultOverridesPath(), "overrides file")
}

func runApps(cmd *cobra.Command, args []string) error {
	casks, err := apps.LoadCasks()
	if err != nil {
		return fmt.Errorf("asking Homebrew about casks: %w", err)
	}
	overrides, err := apps.LoadOverrides(overridesPath)
	if err != nil {
		return err
	}
	list, err := apps.Scan(appsDir, casks, overrides)
	if err != nil {
		return err
	}
	if jsonOut {
		return json.NewEncoder(os.Stdout).Encode(list)
	}
	printApps(list, apps.NotInstalled(overrides, list))
	return nil
}

func printApps(list []apps.App, missing []apps.Missing) {
	groups := []struct {
		updater apps.Updater
		title   string
		fix     string // the one command for the whole group, if there is one
	}{
		{apps.Unknown, "Nobody knows who updates these", ""},
		{apps.Manual, "You update these by hand", ""},
		{apps.Brew, "Homebrew updates these", "brew upgrade"},
		{apps.Self, "These update themselves", ""},
		{apps.MacOS, "Software Update handles these", "System Settings > General > Software Update"},
		{apps.AppStore, "The App Store updates these", "mas upgrade"},
	}
	unknown, casks := 0, 0
	for _, g := range groups {
		var rows []string
		for _, a := range list {
			if a.Updater != g.updater {
				continue
			}
			if a.Updater == apps.Unknown {
				unknown++
			}
			row := fmt.Sprintf("  %-32s %-18s", a.Name, a.Version)
			if a.Cask != "" {
				row += styleDim.Render(" brew cask")
				casks++
			}
			if a.Updater == apps.Manual && a.Reason != "" { // where it came from, from the overrides
				row += styleDim.Render(" " + strings.TrimPrefix(a.Reason, "override: "))
			}
			rows = append(rows, row)
		}
		if len(rows) == 0 {
			continue
		}
		title := styleAccent
		if g.updater == apps.Unknown {
			title = styleWarn
		}
		fmt.Println(title.Render(fmt.Sprintf("%s (%d)", g.title, len(rows))))
		fmt.Println(strings.Join(rows, "\n"))
		if g.fix != "" {
			fmt.Println("  " + line("→ "+g.fix))
		}
		fmt.Println()
	}
	if len(missing) > 0 {
		fmt.Println(styleAccent.Render(fmt.Sprintf("Not installed here, but in the overrides (%d)", len(missing))))
		for _, m := range missing {
			fmt.Printf("  %-32s %-18s %s\n", m.Name, m.Updater, styleDim.Render(m.Note))
		}
		fmt.Println()
	}
	if unknown > 0 {
		fmt.Println("  " + line(fmt.Sprintf("→ name them in %s (koizumi apps --help)", tilde(overridesPath))))
		fmt.Println()
	}
	if casks > 0 {
		fmt.Println(footnote("brew cask: Homebrew installed it; the app still updates itself, so plain brew upgrade leaves it alone"))
		fmt.Println()
	}
}
