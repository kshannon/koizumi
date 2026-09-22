# koizumi

Is my software up to date, and does this machine still match my dotfiles?

koizumi looks at one machine and reports three things:

- **Out of date.** What is behind the latest version its own updater offers: Homebrew,
  the App Store, macOS.
- **Who updates what.** For every app, which updater is responsible: the App Store,
  Homebrew, the app itself, or nobody.
- **Drift.** Whether the machine still matches the dotfiles repo: is everything the
  Brewfile lists installed, and do the files in your home folder match what the repo says.

It never installs or updates software. Where something needs doing, it prints the command
to run. The one thing it does do is move the dotfiles: `pull` fetches the repo and applies
it (chezmoi asks before overwriting anything), `push` pushes it, offering one commit for
anything uncommitted first.

## Install

You need Go 1.27 or newer (`brew install go`). Then:

```bash
go env -w GOBIN="$HOME/.local/bin"        # once: install into a folder already on your PATH
export GOPRIVATE=github.com/kshannon/koizumi   # once, in your shell files: fetch straight from GitHub
go install github.com/kshannon/koizumi@main
```

Skip the first line if `~/go/bin` is already on your `PATH`. Without `GOPRIVATE`, `go install`
goes through Google's module proxy and checksum log, which take a while to learn about a fresh
commit ("404 Not Found" right after a push). To update, run the last line again; `koizumi`
tells you when it is behind. A Homebrew tap will replace this once there is a tagged release.

## Use

```bash
koizumi               # the dashboard: only what needs attention (--cached for the last check)
koizumi outdated      # what is behind: Homebrew, App Store, macOS, with the command to run
koizumi dotfiles      # files in ~ vs the repo (chezmoi), and the repo vs its remote (git)
koizumi brew          # Brewfile vs what is installed: missing and extra
koizumi apps          # every app in /Applications and who updates it
koizumi pull          # git pull --ff-only, then chezmoi apply (asks before overwriting)
koizumi push          # git push; with uncommitted changes it drafts one commit and asks first
koizumi --json ...    # any command as data
koizumi --help        # the full reference for every command
```

`koizumi <command> --help` explains exactly what each command checks and how it decides.
Apps that leave no trace of their updater (Microsoft AutoUpdate, Steam) go in
`~/.config/koizumi/overrides`; `koizumi apps --help` shows the format.

## The line in a new terminal

Two steps. Install the background check, which runs `koizumi check` at 09:00, 15:00 and
login and caches the result:

```bash
koizumi setup
```

Then add one line to `~/.zshrc`:

```bash
command -v koizumi >/dev/null && koizumi motd
```

`motd` only reads the cache, so it costs nothing. It prints one line when something needs
attention, or when the last check is more than a day and a half old, and nothing otherwise.

## Where the data comes from

Every command's `--help` states exactly what it reads and how it decides. In short:

| Command | Reads | Decides with |
|---|---|---|
| `apps` | each app's `Info.plist` and bundle, `brew info --cask --json=v2 --installed`, `~/.config/koizumi/overrides` | six rules, in order (`koizumi apps --help`) |
| `outdated` | `brew outdated --json=v2`, `mas outdated`, Software Update's own plist | each updater's idea of "latest"; never `--greedy` |
| `dotfiles` | `chezmoi status`, `git status`, `git rev-list @{u}...HEAD`, `FETCH_HEAD`'s age | drift if anything differs; it never fetches |
| `brew` | the Brewfile(s), `brew list --installed-on-request`, `brew list --cask`, `brew tap` | two-way set difference on names |
| `check` / dashboard | all of the above | severity order: errors, macOS, dotfiles, Brewfile, Homebrew, App Store, apps |

The background job runs `check` through your login shell (`$SHELL -lc`) so it sees the same
environment you do. That matters: Homebrew keeps the list of third-party taps you trust
under `$XDG_CONFIG_HOME`, and with an empty environment it silently leaves those taps'
formulae out of its answers. Nothing about your machine is hard-coded in koizumi; the
per-machine facts live in your shell files and the overrides file.

## Status

All commands work. macOS only: `setup` knows launchd, and `apps`, `outdated` read macOS
things. On Linux the checks that make sense still run (`dotfiles`, `brew`).

## Build from source

```bash
git clone https://github.com/kshannon/koizumi
cd koizumi
go build -o koizumi .
go test ./...
```
