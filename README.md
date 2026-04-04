<p align="center">
  <img src="assets/icon.png" width="128" alt="meow">
  <h1 align="center">meow</h1>
  <p align="center">Upload files to <a href="https://catbox.moe">catbox.moe</a> & <a href="https://litterbox.catbox.moe">litterbox</a> from the terminal</p>
</p>

<p align="center">
  <img src="assets/demo.gif" alt="meow demo" width="800">
</p>

## Install

```bash
brew install aayush9029/tap/meow
```

Or tap first:

```bash
brew tap aayush9029/tap
brew install meow
```

## Usage

```bash
meow screenshot.png                # Upload file, copy URL to clipboard
meow photo.jpg video.mp4           # Upload multiple files with progress bar
meow url https://example.com/i.png # Re-host from URL
meow temp -t 1h debug.log          # Temporary upload (1h/12h/24h/72h)
meow delete abc123.png             # Delete a file
meow album create --title "pics" a.jpg b.jpg
meow auth <userhash>               # Save credentials
```

| Command | Description |
|---|---|
| `<file> [file...]` | Upload files (default action) |
| `url <url>` | Re-host a file from a URL |
| `temp [-t time] <file>` | Temporary upload via Litterbox (default: 24h) |
| `delete <filename>...` | Delete files (requires auth) |
| `album <subcommand>` | Create, edit, add to, remove from, or delete albums |
| `auth [userhash]` | Set or view your catbox userhash |
| `--anon` | Upload anonymously (skip saved userhash) |

## License

MIT — Aayush Pokharel
