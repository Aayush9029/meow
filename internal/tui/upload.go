package tui

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProgressMsg is sent from the upload goroutine to update the progress bar.
type ProgressMsg struct {
	BytesRead int64
	Total     int64
}

// UploadDoneMsg signals a single file upload completed.
type UploadDoneMsg struct {
	URL      string
	Filename string
}

// UploadErrMsg signals a single file upload failed.
type UploadErrMsg struct {
	Err      error
	Filename string
}

var (
	fileStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	counterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	doneStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	failStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	urlStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

type fileResult struct {
	filename string
	url      string
	err      error
}

// UploadModel manages progress bar display for one or more file uploads.
type UploadModel struct {
	progress progress.Model
	files    []string
	current  int
	percent  float64
	results  []fileResult
	done     bool
	width    int
	uploadFn func(path string) tea.Cmd // returns a tea.Cmd that performs the upload
}

// NewUploadModel creates an upload model for the given file paths.
// uploadFn is called for each file — it should return a tea.Cmd that performs
// the upload and sends ProgressMsg updates followed by UploadDoneMsg/UploadErrMsg.
func NewUploadModel(files []string, uploadFn func(path string) tea.Cmd) *UploadModel {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)
	return &UploadModel{
		progress: p,
		files:    files,
		uploadFn: uploadFn,
	}
}

func (m *UploadModel) Init() tea.Cmd {
	if len(m.files) == 0 {
		return tea.Quit
	}
	return m.uploadFn(m.files[0])
}

func (m *UploadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		barWidth := msg.Width - 30
		if barWidth < 20 {
			barWidth = 20
		}
		if barWidth > 60 {
			barWidth = 60
		}
		m.progress.Width = barWidth

	case ProgressMsg:
		if msg.Total > 0 {
			m.percent = float64(msg.BytesRead) / float64(msg.Total)
		}
		return m, nil

	case UploadDoneMsg:
		m.results = append(m.results, fileResult{
			filename: msg.Filename,
			url:      msg.URL,
		})
		m.percent = 0
		m.current++
		if m.current < len(m.files) {
			return m, m.uploadFn(m.files[m.current])
		}
		m.done = true
		return m, tea.Quit

	case UploadErrMsg:
		m.results = append(m.results, fileResult{
			filename: msg.Filename,
			err:      msg.Err,
		})
		m.percent = 0
		m.current++
		if m.current < len(m.files) {
			return m, m.uploadFn(m.files[m.current])
		}
		m.done = true
		return m, tea.Quit

	case progress.FrameMsg:
		model, cmd := m.progress.Update(msg)
		m.progress = model.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func (m *UploadModel) View() string {
	var s string

	// Show completed results.
	for _, r := range m.results {
		if r.err != nil {
			s += failStyle.Render("  ✗ "+r.filename) + " " + urlStyle.Render(r.err.Error()) + "\n"
		} else {
			s += doneStyle.Render("  ✓ "+r.filename) + " " + urlStyle.Render("→ "+r.url) + "\n"
		}
	}

	// Show current upload progress.
	if !m.done && m.current < len(m.files) {
		name := filepath.Base(m.files[m.current])
		counter := ""
		if len(m.files) > 1 {
			counter = counterStyle.Render(fmt.Sprintf("[%d/%d] ", m.current+1, len(m.files)))
		}
		s += fmt.Sprintf("  → %s%s\n", counter, fileStyle.Render(name))
		s += fmt.Sprintf("  %s %s\n", m.progress.ViewAs(m.percent), counterStyle.Render(fmt.Sprintf("%.0f%%", m.percent*100)))
	}

	return s
}

// Results returns the accumulated upload results after the model finishes.
func (m *UploadModel) Results() []fileResult {
	return m.results
}

// URLs returns all successful upload URLs.
func (m *UploadModel) URLs() []string {
	var urls []string
	for _, r := range m.results {
		if r.err == nil {
			urls = append(urls, r.url)
		}
	}
	return urls
}

// HasErrors returns true if any upload failed.
func (m *UploadModel) HasErrors() bool {
	for _, r := range m.results {
		if r.err != nil {
			return true
		}
	}
	return false
}
