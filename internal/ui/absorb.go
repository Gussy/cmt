package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gussy/cmt/internal/ai"
	"github.com/gussy/cmt/internal/git"
)

// AbsorbReviewModel represents the model for the absorb review UI.
type AbsorbReviewModel struct {
	assignments      []ai.HunkAssignment
	unmatched        []git.Hunk
	commits          []git.CommitInfo
	currentIndex     int
	viewport         viewport.Model
	feedback         textarea.Model
	showAlternatives bool
	selectedAlt      int
	width            int
	height           int
	ready            bool
	accepted         bool
	cancelled        bool
	mode             string // "review", "alternatives", "feedback"
	modifications    map[int]string // Track modified assignments (index -> new SHA).
}

// absorbKeyMap defines the key bindings for the absorb review.
type absorbKeyMap struct {
	Up          key.Binding
	Down        key.Binding
	Accept      key.Binding
	Cancel      key.Binding
	Alternatives key.Binding
	Unassign    key.Binding
	NextHunk    key.Binding
	PrevHunk    key.Binding
	Help        key.Binding
}

var absorbKeys = absorbKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "scroll up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "scroll down"),
	),
	Accept: key.NewBinding(
		key.WithKeys("y", "enter"),
		key.WithHelp("y/enter", "accept assignments"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("n", "q"),
		key.WithHelp("n/q", "cancel"),
	),
	Alternatives: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "show alternatives"),
	),
	Unassign: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "unassign hunk"),
	),
	NextHunk: key.NewBinding(
		key.WithKeys("tab", "right", "l"),
		key.WithHelp("tab/→", "next hunk"),
	),
	PrevHunk: key.NewBinding(
		key.WithKeys("shift+tab", "left", "h"),
		key.WithHelp("shift+tab/←", "prev hunk"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}

// NewAbsorbReviewModel creates a new absorb review model.
func NewAbsorbReviewModel(resp *ai.AbsorbResponse, commits []git.CommitInfo) AbsorbReviewModel {
	// Initialize viewport.
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	// Initialize feedback textarea.
	ta := textarea.New()
	ta.Placeholder = "Enter feedback or press ESC to cancel..."
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.Focus()

	return AbsorbReviewModel{
		assignments:   resp.Assignments,
		unmatched:     resp.UnmatchedHunks,
		commits:       commits,
		viewport:      vp,
		feedback:      ta,
		mode:          "review",
		modifications: make(map[int]string),
		currentIndex:  0,
	}
}

// Init initializes the model.
func (m AbsorbReviewModel) Init() tea.Cmd {
	return tea.EnterAltScreen
}

// Update handles messages and updates the model.
func (m AbsorbReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update viewport size.
		headerHeight := 8
		footerHeight := 4
		verticalMargins := headerHeight + footerHeight
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - verticalMargins

		if !m.ready {
			m.viewport.SetContent(m.renderContent())
			m.ready = true
		}

	case tea.KeyMsg:
		switch m.mode {
		case "review":
			switch {
			case key.Matches(msg, absorbKeys.Accept):
				m.accepted = true
				return m, tea.Quit

			case key.Matches(msg, absorbKeys.Cancel):
				m.cancelled = true
				return m, tea.Quit

			case key.Matches(msg, absorbKeys.NextHunk):
				if m.currentIndex < len(m.assignments)-1 {
					m.currentIndex++
					m.viewport.SetContent(m.renderContent())
				}

			case key.Matches(msg, absorbKeys.PrevHunk):
				if m.currentIndex > 0 {
					m.currentIndex--
					m.viewport.SetContent(m.renderContent())
				}

			case key.Matches(msg, absorbKeys.Alternatives):
				if m.currentIndex < len(m.assignments) &&
					len(m.assignments[m.currentIndex].Alternatives) > 0 {
					m.mode = "alternatives"
					m.selectedAlt = 0
					m.viewport.SetContent(m.renderAlternatives())
				}

			case key.Matches(msg, absorbKeys.Unassign):
				if m.currentIndex < len(m.assignments) {
					// Move assignment to unmatched.
					assignment := m.assignments[m.currentIndex]
					m.unmatched = append(m.unmatched, assignment.Hunk)

					// Remove from assignments.
					m.assignments = append(
						m.assignments[:m.currentIndex],
						m.assignments[m.currentIndex+1:]...,
					)

					// Adjust current index.
					if m.currentIndex >= len(m.assignments) && m.currentIndex > 0 {
						m.currentIndex--
					}

					m.viewport.SetContent(m.renderContent())
				}

			case key.Matches(msg, absorbKeys.Up):
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)

			case key.Matches(msg, absorbKeys.Down):
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)

			case msg.String() == "ctrl+c":
				m.cancelled = true
				return m, tea.Quit
			}

		case "alternatives":
			switch msg.String() {
			case "up", "k":
				if m.selectedAlt > 0 {
					m.selectedAlt--
					m.viewport.SetContent(m.renderAlternatives())
				}

			case "down", "j":
				assignment := m.assignments[m.currentIndex]
				if m.selectedAlt < len(assignment.Alternatives) {
					m.selectedAlt++
					m.viewport.SetContent(m.renderAlternatives())
				}

			case "enter":
				// Apply selected alternative.
				if m.selectedAlt > 0 && m.selectedAlt <= len(m.assignments[m.currentIndex].Alternatives) {
					alt := m.assignments[m.currentIndex].Alternatives[m.selectedAlt-1]
					m.assignments[m.currentIndex].CommitSHA = alt.CommitSHA
					m.assignments[m.currentIndex].CommitMessage = alt.CommitMessage
					m.assignments[m.currentIndex].Confidence = alt.Confidence
					m.assignments[m.currentIndex].Reasoning = alt.Reasoning
					m.modifications[m.currentIndex] = alt.CommitSHA
				}
				m.mode = "review"
				m.viewport.SetContent(m.renderContent())

			case "q", "esc":
				m.mode = "review"
				m.viewport.SetContent(m.renderContent())
			}
		}

	default:
		// Handle viewport updates.
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI.
func (m AbsorbReviewModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1)

	b.WriteString(headerStyle.Render("🔍 Absorb Review"))
	b.WriteString("\n\n")

	// Stats bar
	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	stats := fmt.Sprintf(
		"Hunk %d/%d | Assigned: %d | Unmatched: %d",
		m.currentIndex+1,
		len(m.assignments),
		len(m.assignments),
		len(m.unmatched),
	)

	if _, modified := m.modifications[m.currentIndex]; modified {
		stats += " [MODIFIED]"
	}

	b.WriteString(statsStyle.Render(stats))
	b.WriteString("\n\n")

	// Viewport
	b.WriteString(m.viewport.View())
	b.WriteString("\n\n")

	// Controls
	var controls string
	if m.mode == "review" {
		controls = "[y] Accept  [n] Cancel  [←/→] Navigate  [a] Alternatives  [u] Unassign  [?] Help"
	} else if m.mode == "alternatives" {
		controls = "[↑/↓] Select  [enter] Apply  [esc] Cancel"
	}

	controlsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	b.WriteString(controlsStyle.Render(controls))

	return b.String()
}

// renderContent renders the main content for the current assignment.
func (m *AbsorbReviewModel) renderContent() string {
	if len(m.assignments) == 0 {
		return "No assignments to review."
	}

	if m.currentIndex >= len(m.assignments) {
		return "No more assignments."
	}

	assignment := m.assignments[m.currentIndex]
	var b strings.Builder

	// Assignment header
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	b.WriteString(titleStyle.Render(fmt.Sprintf("Assignment for %s", assignment.Hunk.FilePath)))
	b.WriteString("\n\n")

	// Target commit info
	commitStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	b.WriteString("Target Commit:\n")
	b.WriteString(commitStyle.Render(fmt.Sprintf("  %s: %s\n",
		assignment.CommitSHA[:8],
		assignment.CommitMessage,
	)))
	b.WriteString("\n")

	// Confidence
	confidenceStyle := lipgloss.NewStyle()
	if assignment.Confidence >= 0.8 {
		confidenceStyle = confidenceStyle.Foreground(lipgloss.Color("82"))
	} else if assignment.Confidence >= 0.5 {
		confidenceStyle = confidenceStyle.Foreground(lipgloss.Color("214"))
	} else {
		confidenceStyle = confidenceStyle.Foreground(lipgloss.Color("196"))
	}

	b.WriteString(fmt.Sprintf("Confidence: %s\n",
		confidenceStyle.Render(fmt.Sprintf("%.1f%%", assignment.Confidence*100))))
	b.WriteString("\n")

	// Reasoning
	if assignment.Reasoning != "" {
		reasonStyle := lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("241"))

		b.WriteString("Reasoning:\n")
		b.WriteString(reasonStyle.Render(fmt.Sprintf("  %s\n", assignment.Reasoning)))
		b.WriteString("\n")
	}

	// Alternatives indicator
	if len(assignment.Alternatives) > 0 {
		altStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

		b.WriteString(altStyle.Render(fmt.Sprintf("ℹ %d alternative(s) available. Press [a] to view.\n",
			len(assignment.Alternatives))))
		b.WriteString("\n")
	}

	// Hunk content
	b.WriteString("Hunk Content:\n")
	b.WriteString("─────────────\n")

	// Format diff with syntax highlighting
	diffStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	addStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("82"))
	removeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	lines := strings.Split(assignment.Hunk.Content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			b.WriteString(addStyle.Render(line))
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			b.WriteString(removeStyle.Render(line))
		} else if strings.HasPrefix(line, "@@") {
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).
				Bold(true).
				Render(line))
		} else {
			b.WriteString(diffStyle.Render(line))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderAlternatives renders the alternatives selection view.
func (m *AbsorbReviewModel) renderAlternatives() string {
	if m.currentIndex >= len(m.assignments) {
		return "No assignment selected."
	}

	assignment := m.assignments[m.currentIndex]
	var b strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	b.WriteString(titleStyle.Render("Select Alternative Assignment"))
	b.WriteString("\n\n")

	// Current assignment (option 0)
	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("82"))
	normalStyle := lipgloss.NewStyle()

	style := normalStyle
	if m.selectedAlt == 0 {
		style = selectedStyle
		b.WriteString("▶ ")
	} else {
		b.WriteString("  ")
	}

	b.WriteString(style.Render(fmt.Sprintf("[Current] %s: %s (%.1f%%)\n",
		assignment.CommitSHA[:8],
		assignment.CommitMessage,
		assignment.Confidence*100,
	)))

	// Alternatives
	for i, alt := range assignment.Alternatives {
		style = normalStyle
		if m.selectedAlt == i+1 {
			style = selectedStyle
			b.WriteString("▶ ")
		} else {
			b.WriteString("  ")
		}

		b.WriteString(style.Render(fmt.Sprintf("[Alt %d] %s: %s (%.1f%%)\n",
			i+1,
			alt.CommitSHA[:8],
			alt.CommitMessage,
			alt.Confidence*100,
		)))

		if alt.Reasoning != "" && m.selectedAlt == i+1 {
			reasonStyle := lipgloss.NewStyle().
				Italic(true).
				Foreground(lipgloss.Color("241"))
			b.WriteString(reasonStyle.Render(fmt.Sprintf("    → %s\n", alt.Reasoning)))
		}
	}

	return b.String()
}

// GetResult returns whether the review was accepted and any modifications.
func (m *AbsorbReviewModel) GetResult() (bool, *ai.AbsorbResponse) {
	if m.cancelled {
		return false, nil
	}

	if !m.accepted {
		return false, nil
	}

	// Build modified response if there were changes.
	if len(m.modifications) > 0 {
		resp := &ai.AbsorbResponse{
			Assignments:    m.assignments,
			UnmatchedHunks: m.unmatched,
		}
		return true, resp
	}

	return true, nil
}

// ShowAbsorbReview shows the interactive absorb review UI.
func ShowAbsorbReview(resp *ai.AbsorbResponse, commits []git.CommitInfo) (bool, *ai.AbsorbResponse, error) {
	model := NewAbsorbReviewModel(resp, commits)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return false, nil, err
	}

	if m, ok := finalModel.(AbsorbReviewModel); ok {
		accepted, modifiedResp := m.GetResult()
		return accepted, modifiedResp, nil
	}

	return false, nil, nil
}