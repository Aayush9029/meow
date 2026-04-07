package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Aayush9029/meow/internal/api"
	"github.com/Aayush9029/meow/internal/config"
	"github.com/Aayush9029/meow/internal/tui"
	"github.com/Aayush9029/meow/internal/ui"
)

func main() {
	config.MigrateOld()

	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		os.Exit(0)
	}

	// Extract global flags.
	anon := false
	var filtered []string
	for _, a := range args {
		if a == "--anon" {
			anon = true
		} else {
			filtered = append(filtered, a)
		}
	}
	args = filtered

	if len(args) == 0 {
		printUsage()
		os.Exit(0)
	}

	switch args[0] {
	case "--version", "-v", "version":
		fmt.Printf("meow %s\n", config.Version)
	case "--help", "-h", "help":
		printUsage()
	case "auth":
		cmdAuth(args[1:])
	case "url":
		cmdURLUpload(args[1:], anon)
	case "delete", "rm":
		cmdDelete(args[1:])
	case "temp", "tmp":
		cmdTemp(args[1:])
	case "album":
		cmdAlbum(args[1:])
	default:
		cmdUpload(args, anon)
	}
}

// ─── Upload ──────────────────────────────────────────────────────────────────

func cmdUpload(args []string, anon bool) {
	if len(args) == 0 {
		ui.Fatalf("no files specified\nUsage: meow <file|folder> [file|folder...]")
	}

	// Validate all paths exist, zip directories.
	paths, cleanup := preparePaths(args, config.MaxFileSize, "200 MB")
	defer cleanup()

	cfg := config.Load()
	client := api.NewClient()

	fields := map[string]string{"reqtype": "fileupload"}
	if cfg.Userhash != "" && !anon {
		fields["userhash"] = cfg.Userhash
	}

	if ui.IsTTY() {
		ui.Header("meow")
		ui.Dimf("  v%s · catbox.moe", config.Version)
		fmt.Fprintln(os.Stderr)

		uploadWithProgress(client, paths, fields, config.CatboxAPI)
	} else {
		// Piped mode: no decoration, just URLs.
		for _, path := range paths {
			url, err := client.PostMultipart(config.CatboxAPI, copyFields(fields), "fileToUpload", path, nil)
			if err != nil {
				ui.Fatalf("upload failed: %v", err)
			}
			fmt.Println(url)
		}
	}
}

// ─── URL Upload ──────────────────────────────────────────────────────────────

func cmdURLUpload(args []string, anon bool) {
	if len(args) == 0 {
		ui.Fatalf("no URL specified\nUsage: meow url <url>")
	}

	cfg := config.Load()
	client := api.NewClient()

	fields := map[string]string{
		"reqtype": "urlupload",
		"url":     args[0],
	}
	if cfg.Userhash != "" && !anon {
		fields["userhash"] = cfg.Userhash
	}

	if ui.IsTTY() {
		ui.Header("meow")
		ui.Dimf("  v%s · catbox.moe", config.Version)
		fmt.Fprintln(os.Stderr)

		model := tui.NewSpinnerModel("uploading from URL...", func() tea.Msg {
			url, err := client.PostMultipart(config.CatboxAPI, fields, "", "", nil)
			if err != nil {
				return tui.ActionErrMsg{Err: err}
			}
			return tui.ActionDoneMsg{Result: url}
		})

		p := tea.NewProgram(model, tea.WithOutput(os.Stderr))
		final, err := p.Run()
		if err != nil {
			ui.Fatalf("TUI error: %v", err)
		}

		m := final.(*tui.SpinnerModel)
		if m.Err != nil {
			ui.Fatalf("upload failed: %v", m.Err)
		}

		ui.Success(fmt.Sprintf("uploaded → %s", m.Result))
		fmt.Println(m.Result)
		ui.Clipboard(m.Result)
	} else {
		url, err := client.PostMultipart(config.CatboxAPI, fields, "", "", nil)
		if err != nil {
			ui.Fatalf("upload failed: %v", err)
		}
		fmt.Println(url)
	}
}

// ─── Delete ──────────────────────────────────────────────────────────────────

func cmdDelete(args []string) {
	if len(args) == 0 {
		ui.Fatalf("no files specified\nUsage: meow delete <filename> [filename...]")
	}

	cfg := config.Load()
	if cfg.Userhash == "" {
		ui.Fatalf("userhash required for deletion\nRun: meow auth <userhash>")
	}

	filenames := make([]string, len(args))
	for i, a := range args {
		filenames[i] = extractFilename(a)
	}

	fields := map[string]string{
		"reqtype":  "deletefiles",
		"userhash": cfg.Userhash,
		"files":    strings.Join(filenames, " "),
	}

	client := api.NewClient()

	if ui.IsTTY() {
		model := tui.NewSpinnerModel(
			fmt.Sprintf("deleting %d file(s)...", len(filenames)),
			func() tea.Msg {
				result, err := client.PostMultipart(config.CatboxAPI, fields, "", "", nil)
				if err != nil {
					return tui.ActionErrMsg{Err: err}
				}
				return tui.ActionDoneMsg{Result: result}
			},
		)

		p := tea.NewProgram(model, tea.WithOutput(os.Stderr))
		final, err := p.Run()
		if err != nil {
			ui.Fatalf("TUI error: %v", err)
		}

		m := final.(*tui.SpinnerModel)
		if m.Err != nil {
			ui.Fatalf("delete failed: %v", m.Err)
		}
		ui.Success(fmt.Sprintf("deleted %d file(s)", len(filenames)))
	} else {
		result, err := client.PostMultipart(config.CatboxAPI, fields, "", "", nil)
		if err != nil {
			ui.Fatalf("delete failed: %v", err)
		}
		if result != "" {
			fmt.Println(result)
		}
	}
}

// ─── Temp (Litterbox) ───────────────────────────────────────────────────────

func cmdTemp(args []string) {
	duration := "24h"
	var files []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-t", "--time":
			if i+1 >= len(args) {
				ui.Fatalf("--time requires a value (1h, 12h, 24h, 72h)")
			}
			i++
			duration = args[i]
		default:
			files = append(files, args[i])
		}
	}

	validTimes := map[string]bool{"1h": true, "12h": true, "24h": true, "72h": true}
	if !validTimes[duration] {
		ui.Fatalf("invalid time %q — must be 1h, 12h, 24h, or 72h", duration)
	}

	if len(files) == 0 {
		ui.Fatalf("no files specified\nUsage: meow temp [-t time] <file|folder> [file|folder...]")
	}

	paths, cleanup := preparePaths(files, config.MaxLBSize, "1 GB")
	defer cleanup()

	client := api.NewClient()
	fields := map[string]string{
		"reqtype": "fileupload",
		"time":    duration,
	}

	if ui.IsTTY() {
		ui.Header("meow")
		ui.Dimf("  v%s · litterbox (expires in %s)", config.Version, duration)
		fmt.Fprintln(os.Stderr)

		uploadWithProgress(client, paths, fields, config.LitterboxAPI)
	} else {
		for _, path := range paths {
			url, err := client.PostMultipart(config.LitterboxAPI, copyFields(fields), "fileToUpload", path, nil)
			if err != nil {
				ui.Fatalf("upload failed: %v", err)
			}
			fmt.Println(url)
		}
	}
}

// ─── Album ───────────────────────────────────────────────────────────────────

func cmdAlbum(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s%sAlbum commands:%s\n", ui.Bold, ui.Cyan, ui.Reset)
		fmt.Fprintf(os.Stderr, "  meow album create [--title T] [--desc D] <file> [file...]\n")
		fmt.Fprintf(os.Stderr, "  meow album delete <short>\n")
		fmt.Fprintf(os.Stderr, "  meow album add <short> <file> [file...]\n")
		fmt.Fprintf(os.Stderr, "  meow album remove <short> <file> [file...]\n")
		fmt.Fprintf(os.Stderr, "  meow album edit <short> [--title T] [--desc D] <file> [file...]\n")
		os.Exit(0)
	}

	cfg := config.Load()
	client := api.NewClient()

	switch args[0] {
	case "create":
		albumCreate(cfg, client, args[1:])
	case "delete", "rm":
		albumDelete(cfg, client, args[1:])
	case "add":
		albumAddRemove(cfg, client, "addtoalbum", args[1:])
	case "remove":
		albumAddRemove(cfg, client, "removefromalbum", args[1:])
	case "edit":
		albumEdit(cfg, client, args[1:])
	default:
		ui.Fatalf("unknown album command %q", args[0])
	}
}

func albumCreate(cfg config.Config, client *api.Client, args []string) {
	title, desc := "", ""
	var files []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--title":
			if i+1 < len(args) {
				i++
				title = args[i]
			}
		case "--desc":
			if i+1 < len(args) {
				i++
				desc = args[i]
			}
		default:
			files = append(files, extractFilename(args[i]))
		}
	}

	if len(files) == 0 {
		ui.Fatalf("no files specified for album")
	}

	fields := map[string]string{
		"reqtype": "createalbum",
		"title":   title,
		"desc":    desc,
		"files":   strings.Join(files, " "),
	}
	if cfg.Userhash != "" {
		fields["userhash"] = cfg.Userhash
	}

	url, err := client.PostMultipart(config.CatboxAPI, fields, "", "", nil)
	if err != nil {
		ui.Fatalf("album creation failed: %v", err)
	}

	ui.Success(fmt.Sprintf("album created → %s", url))
	fmt.Println(url)
	ui.Clipboard(url)
}

func albumDelete(cfg config.Config, client *api.Client, args []string) {
	if len(args) == 0 {
		ui.Fatalf("no album short code specified")
	}
	if cfg.Userhash == "" {
		ui.Fatalf("userhash required\nRun: meow auth <userhash>")
	}

	fields := map[string]string{
		"reqtype":  "deletealbum",
		"userhash": cfg.Userhash,
		"short":    args[0],
	}

	_, err := client.PostMultipart(config.CatboxAPI, fields, "", "", nil)
	if err != nil {
		ui.Fatalf("album deletion failed: %v", err)
	}
	ui.Success(fmt.Sprintf("album %s deleted", args[0]))
}

func albumAddRemove(cfg config.Config, client *api.Client, reqtype string, args []string) {
	if len(args) < 2 {
		ui.Fatalf("usage: meow album add|remove <short> <file> [file...]")
	}
	if cfg.Userhash == "" {
		ui.Fatalf("userhash required\nRun: meow auth <userhash>")
	}

	short := args[0]
	files := make([]string, len(args)-1)
	for i, a := range args[1:] {
		files[i] = extractFilename(a)
	}

	fields := map[string]string{
		"reqtype":  reqtype,
		"userhash": cfg.Userhash,
		"short":    short,
		"files":    strings.Join(files, " "),
	}

	_, err := client.PostMultipart(config.CatboxAPI, fields, "", "", nil)
	if err != nil {
		ui.Fatalf("operation failed: %v", err)
	}

	action := "added to"
	if reqtype == "removefromalbum" {
		action = "removed from"
	}
	ui.Success(fmt.Sprintf("%d file(s) %s album %s", len(files), action, short))
}

func albumEdit(cfg config.Config, client *api.Client, args []string) {
	if len(args) == 0 {
		ui.Fatalf("usage: meow album edit <short> [--title T] [--desc D] <file> [file...]")
	}
	if cfg.Userhash == "" {
		ui.Fatalf("userhash required\nRun: meow auth <userhash>")
	}

	short := args[0]
	title, desc := "", ""
	var files []string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--title":
			if i+1 < len(args) {
				i++
				title = args[i]
			}
		case "--desc":
			if i+1 < len(args) {
				i++
				desc = args[i]
			}
		default:
			files = append(files, extractFilename(args[i]))
		}
	}

	fields := map[string]string{
		"reqtype":  "editalbum",
		"userhash": cfg.Userhash,
		"short":    short,
		"title":    title,
		"desc":     desc,
		"files":    strings.Join(files, " "),
	}

	_, err := client.PostMultipart(config.CatboxAPI, fields, "", "", nil)
	if err != nil {
		ui.Fatalf("album edit failed: %v", err)
	}
	ui.Success(fmt.Sprintf("album %s updated", short))
}

// ─── Auth ────────────────────────────────────────────────────────────────────

func cmdAuth(args []string) {
	if len(args) == 0 {
		cfg := config.Load()
		if cfg.Userhash == "" {
			fmt.Fprintf(os.Stderr, "%sNo userhash configured%s\n", ui.Yellow, ui.Reset)
			fmt.Fprintf(os.Stderr, "%sGet yours at: %shttps://catbox.moe/user/manage.php%s\n", ui.Dim, ui.Cyan, ui.Reset)
			fmt.Fprintf(os.Stderr, "Then run: meow auth <userhash>\n")
		} else {
			masked := cfg.Userhash[:4] + strings.Repeat("•", len(cfg.Userhash)-4)
			ui.Success(fmt.Sprintf("authenticated as %s%s%s", ui.Bold, masked, ui.Reset))
		}
		return
	}

	cfg := config.Config{Userhash: args[0]}
	if err := config.Save(cfg); err != nil {
		ui.Fatalf("failed to save config: %v", err)
	}
	ui.Success(fmt.Sprintf("userhash saved to %s", config.Path()))
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func extractFilename(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return name
}

func copyFields(fields map[string]string) map[string]string {
	cp := make(map[string]string, len(fields))
	for k, v := range fields {
		cp[k] = v
	}
	return cp
}

// uploadWithProgress runs a bubbletea program with a progress bar for file uploads (TTY mode).
func uploadWithProgress(client *api.Client, files []string, baseFields map[string]string, apiURL string) {
	var p *tea.Program

	uploadFn := func(path string) tea.Cmd {
		return func() tea.Msg {
			fields := copyFields(baseFields)
			url, err := client.PostMultipart(apiURL, fields, "fileToUpload", path,
				func(bytesRead, total int64) {
					p.Send(tui.ProgressMsg{BytesRead: bytesRead, Total: total})
				},
			)
			if err != nil {
				return tui.UploadErrMsg{Err: err, Filename: filepath.Base(path)}
			}
			return tui.UploadDoneMsg{URL: url, Filename: filepath.Base(path)}
		}
	}

	model := tui.NewUploadModel(files, uploadFn)
	p = tea.NewProgram(model, tea.WithOutput(os.Stderr))

	final, err := p.Run()
	if err != nil {
		ui.Fatalf("TUI error: %v", err)
	}

	m := final.(*tui.UploadModel)
	fmt.Fprintln(os.Stderr) // blank line after progress

	// Print URLs to stdout.
	urls := m.URLs()
	for _, url := range urls {
		fmt.Println(url)
	}

	if len(urls) == 1 {
		ui.Clipboard(urls[0])
	}

	if m.HasErrors() {
		os.Exit(1)
	}
}

// ─── Directory → Zip ────────────────────────────────────────────────────────

// preparePaths validates paths and zips any directories. Returns the resolved
// file paths and a cleanup function that removes temp zips.
func preparePaths(args []string, maxSize int64, maxLabel string) ([]string, func()) {
	var paths []string
	var tmpFiles []string

	for _, p := range args {
		info, err := os.Stat(p)
		if err != nil {
			ui.Fatalf("not found: %s", p)
		}

		if info.IsDir() {
			if ui.IsTTY() {
				ui.Status(fmt.Sprintf("zipping %s/", filepath.Base(p)))
			}
			zipPath, err := zipDirectory(p)
			if err != nil {
				ui.Fatalf("failed to zip %s: %v", p, err)
			}
			tmpFiles = append(tmpFiles, zipPath)
			info, _ = os.Stat(zipPath)
			if info.Size() > maxSize {
				os.Remove(zipPath)
				ui.Fatalf("%s zipped is too large (%s, max %s)", p, ui.HumanSize(info.Size()), maxLabel)
			}
			paths = append(paths, zipPath)
		} else {
			if info.Size() > maxSize {
				ui.Fatalf("%s is too large (%s, max %s)", p, ui.HumanSize(info.Size()), maxLabel)
			}
			paths = append(paths, p)
		}
	}

	cleanup := func() {
		for _, f := range tmpFiles {
			os.Remove(f)
		}
	}
	return paths, cleanup
}

// zipDirectory creates a temporary .zip of the given directory and returns
// the path to the zip file.
func zipDirectory(dir string) (string, error) {
	tmpFile, err := os.CreateTemp("", filepath.Base(dir)+"-*.zip")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	w := zip.NewWriter(tmpFile)
	defer w.Close()

	base := filepath.Dir(dir)
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate

		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// ─── Help ────────────────────────────────────────────────────────────────────

func printUsage() {
	if ui.IsTTY() {
		ui.Header("meow")
		ui.Dimf("  v%s · upload to catbox.moe & litterbox", config.Version)
	}

	fmt.Fprintf(os.Stderr, `
%sUsage:%s
  meow <file|folder> [file|folder...]     Upload files or folders (permanent)
  meow url <url>                          Upload from URL
  meow temp [-t 1h|12h|24h|72h] <path>   Temporary upload (default: 24h)
  meow delete <filename> [filename...]    Delete uploaded files
  meow album <command>                    Album management
  meow auth [userhash]                    Set or view auth

%sFlags:%s
  --anon          Upload anonymously (ignore saved userhash)
  --version, -v   Show version
  --help, -h      Show this help

%sExamples:%s
  %smeow screenshot.png                     # Upload and copy URL%s
  %smeow *.jpg                              # Upload multiple files%s
  %smeow ./my-project                       # Zip folder and upload%s
  %smeow url https://i.imgur.com/x.png      # Re-host from URL%s
  %smeow temp -t 1h debug.log               # Temp upload, expires in 1 hour%s
  %smeow delete abc123.png                  # Delete a file%s

%sConfig:%s %s
`, ui.Bold, ui.Reset,
		ui.Bold, ui.Reset,
		ui.Bold, ui.Reset,
		ui.Dim, ui.Reset,
		ui.Dim, ui.Reset,
		ui.Dim, ui.Reset,
		ui.Dim, ui.Reset,
		ui.Dim, ui.Reset,
		ui.Dim, ui.Reset,
		ui.Dim, ui.Reset, config.Path())
}
