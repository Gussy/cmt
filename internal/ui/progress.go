package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// progressModel represents a progress indicator with a spinner.
type progressModel struct {
	spinner  spinner.Model
	message  string
	done     bool
	quitting bool
}

// ProgressResult contains the result of a progress operation.
type ProgressResult struct {
	Completed bool
	Error     error
}

var (
	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("63"))

	progressTextStyle = lipgloss.NewStyle().
				MarginLeft(1)
)

// newProgressModel creates a new progress model.
func newProgressModel(message string) progressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return progressModel{
		spinner: s,
		message: message,
	}
}

// Init initializes the progress model.
func (m progressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages for the progress model.
func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}

	case progressCompleteMsg:
		m.done = true
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the progress indicator.
func (m progressModel) View() string {
	if m.done {
		return ""
	}

	str := fmt.Sprintf(
		"%s %s",
		m.spinner.View(),
		progressTextStyle.Render(m.message),
	)

	if m.quitting {
		return str + "\n"
	}
	return str
}

// progressCompleteMsg signals that the progress operation is complete.
type progressCompleteMsg struct{}

// CompleteProgress sends a message to complete the progress indicator.
func CompleteProgress() tea.Msg {
	return progressCompleteMsg{}
}

// ShowProgress displays a progress spinner with a message.
// This is a non-blocking function that returns a Program that can be killed.
func ShowProgress(message string) *tea.Program {
	m := newProgressModel(message)
	p := tea.NewProgram(m)

	go func() {
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error running progress: %v\n", err)
		}
	}()

	// Give it a moment to start.
	time.Sleep(50 * time.Millisecond)

	return p
}

// ProgressMessages defines common progress messages.
var ProgressMessages = struct {
	StagingFiles        string
	StagingUpdatedFiles string
	AnalyzingChanges    string
	GeneratingMessage   string
	Regenerating        string
	ScanningSecrets     string
	CreatingCommit      string
	PushingChanges      string
}{
	StagingFiles:        "Staging all changes...",
	StagingUpdatedFiles: "Staging updated files...",
	AnalyzingChanges:    "Analyzing changes...",
	GeneratingMessage:   "Generating commit message with Claude...",
	Regenerating:        "Regenerating with feedback...",
	ScanningSecrets:     "Scanning for secrets...",
	CreatingCommit:      "Creating commit...",
	PushingChanges:      "Pushing to remote...",
}

// SimpleProgress shows a simple inline progress message without Bubble Tea.
// This is useful for quick operations or when we don't want a full TUI.
func SimpleProgress(message string) {
	fmt.Printf("%s %s\n", spinnerStyle.Render("⠋"), message)
}

// ClearProgress clears the previous progress line.
func ClearProgress() {
	fmt.Print("\r" + strings.Repeat(" ", 80) + "\r")
}
