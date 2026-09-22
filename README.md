# koizumi

Is my software up to date, and does this machine still match my dotfiles?

koizumi looks at one machine and reports three things:

- **Out of date.** What is behind the latest version its own updater offers: Homebrew,
  the App Store, macOS.
- **Who updates what.** For every app, which updater is responsible: the App Store,
  Homebrew, the app itself, or nobody.
- **Drift.** Whether the machine still matches the dotfiles repo: is everything the
  Brewfile lists installed, and do the files in your home folder match what the repo says.

It only reports. It never installs, updates or changes anything. Where something needs
doing, it prints the command to run.

## Install

You need Go 1.27 or newer (`brew install go`). Then:

```bash
go install github.com/kshannon/koizumi@latest
```

That puts the `koizumi` binary in `~/go/bin`. Make sure that folder is on your `PATH`.
To update, run the same command again. A Homebrew tap will replace this once there is a
tagged release.

## Use

```bash
koizumi outdated      # what is behind: Homebrew, App Store, macOS, with the command to run
koizumi apps          # every app in /Applications and who updates it
koizumi --json ...    # any command as data
koizumi --help        # the full reference for every command
```

`koizumi <command> --help` explains exactly what each command checks and how it decides.
Apps that leave no trace of their updater (Microsoft AutoUpdate, Steam) go in
`~/.config/koizumi/overrides`; `koizumi apps --help` shows the format.

## Status

Early. `outdated` and `apps` work. `brew`, `dotfiles`, `setup` and the dashboard shown by
`koizumi` with no arguments are designed but not built; each says so and exits 1.

## Build from source

```bash
git clone https://github.com/kshannon/koizumi
cd koizumi
go build -o koizumi .
go test ./...
```
