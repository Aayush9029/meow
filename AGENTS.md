# meow

A Go CLI that uploads files to catbox.moe and Litterbox, copies the URL, and manages albums. It ships through `aayush9029/tap`. Status: shipped.

## Commands

| Task | Command |
|------|---------|
| Build | `go build -o meow .` |
| Run | `go run . file.png` |

The repository has no tests.

## Architecture

- `main.go` dispatches the subcommands with a manual `switch`.
- `internal/api` calls the catbox and Litterbox APIs. `internal/tui` shows the upload spinner and progress.
- `internal/config` stores the catbox `userhash` in `~/.config/meow/config.json`. It moves an old `~/.config/catbox/config.json` file to the new path.

## Rules

- The version is `config.Version` in `internal/config/config.go`, not a `main.version` variable. The tap formula does not inject it. Change `config.Version` before you tag.
- `delete` and album changes need a saved `userhash`. `--anon` uploads without it.
- Litterbox accepts only `1h`, `12h`, `24h`, and `72h`.
