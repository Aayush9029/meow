package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

const (
	Green  = "\033[32m"
	Red    = "\033[31m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
	Dim    = "\033[2m"
	Bold   = "\033[1m"
	Reset  = "\033[0m"
)

func IsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func TermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

func Header(msg string) {
	if !IsTTY() {
		return
	}
	fmt.Fprintf(os.Stderr, "\n%s%s⚡ %s%s\n", Cyan, Bold, msg, Reset)
}

func Success(msg string) {
	fmt.Fprintf(os.Stderr, "%s✓ %s%s\n", Green, msg, Reset)
}

func Error(msg string) {
	fmt.Fprintf(os.Stderr, "%s✗ %s%s\n", Red, msg, Reset)
}

func Status(msg string) {
	fmt.Fprintf(os.Stderr, "%s→ %s%s\n", Dim, msg, Reset)
}

func Dimf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s"+format+"%s\n", append([]any{Dim}, append(a, Reset)...)...)
}

func Fatalf(format string, a ...any) {
	Error(fmt.Sprintf(format, a...))
	os.Exit(1)
}

func Clipboard(url string) {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(url)
	if err := cmd.Run(); err == nil && IsTTY() {
		Dimf("  copied to clipboard")
	}
}

func HumanSize(b int64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
