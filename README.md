# koizumi

Is my software up to date, and who updates it?

koizumi looks at one machine and answers two questions: is anything out of date, and
which updater is responsible for each app: the App Store, Homebrew, the app itself, or
nobody. It only reports. It never installs or updates anything; it prints the command to
run instead.

Named after Itsuki Koizumi, the SOS Brigade member whose job is to notice trouble and
report back.

## Status

Early. Working today:

```
koizumi apps        every app in /Applications and who updates it (--json for data)
```

Designed, not built yet: `outdated`, `brew`, `dotfiles`, `setup`, and the dashboard that
`koizumi` with no arguments will show. Each prints "not built yet" until it exists.

## How `apps` decides

For each `.app` it reads `Info.plist` and the bundle, then asks Homebrew about installed
casks:

| Evidence | Verdict |
|---|---|
| `Contents/_MASReceipt/receipt` | App Store updates it |
| `SUFeedURL` in Info.plist, or a Sparkle / Squirrel / Keystone framework, or an `updater.app` inside the bundle | the app updates itself |
| a Homebrew cask marked `auto_updates` | the app updates itself |
| a Homebrew cask with no updater | Homebrew updates it: `brew upgrade` |
| none of the above | unknown: check by hand |

## Build

```
go build -o koizumi .
go test ./...
```

Requires Go 1.27 or newer. Releases will be published through a Homebrew tap once the
first version is tagged.
