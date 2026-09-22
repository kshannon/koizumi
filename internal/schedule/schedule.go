// Package schedule installs the background check as a macOS launchd agent.
package schedule

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Label is the launchd job name.
const Label = "com.kshannon.koizumi"

// Plist renders the agent: run `<binary> check` at 09:00 and 15:00 and at login.
//
// It runs through the user's login shell (`shell -lc`) so the job sees the same environment
// the user does. launchd's own environment is almost empty: its PATH has no /opt/homebrew/bin
// (a previous daily job silently skipped every Homebrew check for its entire life), and it
// has no XDG_CONFIG_HOME, which is where Homebrew keeps the record of which third-party taps
// you trust; without it brew quietly drops those taps' formulae from its answers.
// An explicit PATH is still set as a floor, in case the shell files do not set one.
func Plist(shell, binary, log string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>-lc</string>
		<string>'%s' check</string>
	</array>
	<key>EnvironmentVariables</key>
	<dict>
		<key>PATH</key>
		<string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
	</dict>
	<key>StartCalendarInterval</key>
	<array>
		<dict><key>Hour</key><integer>9</integer><key>Minute</key><integer>0</integer></dict>
		<dict><key>Hour</key><integer>15</integer><key>Minute</key><integer>0</integer></dict>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, Label, shell, binary, log, log)
}

// PlistPath is ~/Library/LaunchAgents/<Label>.plist.
func PlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

// LogPath is ~/Library/Logs/koizumi.log.
func LogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Logs", "koizumi.log"), nil
}

// Install writes the plist for the given shell and binary and (re)loads it. Safe to run again.
func Install(shell, binary string) (string, error) {
	path, err := PlistPath()
	if err != nil {
		return "", err
	}
	log, err := LogPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(Plist(shell, binary, log)), 0o644); err != nil {
		return "", err
	}
	_ = launchctl("bootout", domain()+"/"+Label) // not loaded yet is fine
	if err := launchctl("bootstrap", domain(), path); err != nil {
		return path, fmt.Errorf("launchctl bootstrap: %w", err)
	}
	return path, nil
}

// Uninstall unloads the agent and removes the plist.
func Uninstall() error {
	path, err := PlistPath()
	if err != nil {
		return err
	}
	_ = launchctl("bootout", domain()+"/"+Label)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Loaded says whether launchd currently has the agent.
func Loaded() bool {
	return launchctl("print", domain()+"/"+Label) == nil
}

func domain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

func launchctl(args ...string) error {
	out, err := exec.Command("launchctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}
