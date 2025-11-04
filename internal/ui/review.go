package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ReviewAction represents the user's decision from the review screen.
type ReviewAction int

const (
	// ReviewAccept means the user accepted the commit message.
	ReviewAccept ReviewAction = iota
	// ReviewReject means the user rejected the commit message.
	ReviewReject
	// ReviewRegenerate means the user wants to regenerate with feedback.
	ReviewRegenerate
	// ReviewEdit means the user wants to manually edit the message.
	ReviewEdit
	// ReviewEditInline means the user wants to edit inline using textarea.
	ReviewEditInline
)

// reviewModel is the Bubble Tea model for the commit review screen.
type reviewModel struct {
	message        string          // The generated commit message.
	diff           string          // The git diff to display.
	viewport       viewport.Model  // Scrollable viewport for diff.
	textarea       textarea.Model  // Textarea for feedback input.
	showFeedback   bool            // Whether to show feedback input.
	editMode       bool            // Whether in inline edit mode.
	editTextarea   textarea.Model  // Textarea for editing message.
	preferExternal bool            // Whether to prefer external editor (based on config).
	action         ReviewAction    // User's final decision.
	feedback       string          // User's feedback for regeneration.
	width          int             // Terminal width.
	height         int             // Terminal height.
	ready          bool            // Whether the model is ready.
	done           bool            // Whether the review is complete.

	// Debug fields
	debugReserved      int // Reserved height calculated
	debugViewportHeight int // Viewport height calculated
}

// Styling definitions.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63")).  // Changed to a more visible cyan color
			MarginBottom(1)

	messageBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	feedbackStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	focusedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205"))
)

// newReviewModel creates a new review model.
func newReviewModel(message, diff string) reviewModel {
	// Create viewport for diff display.
	vp := viewport.New(0, 0)
	vp.SetContent(formatDiff(diff, 0))

	// Create textarea for feedback.
	ta := textarea.New()
	ta.Placeholder = "Enter feedback for regeneration..."
	ta.ShowLineNumbers = false
	ta.SetHeight(5)
	ta.Focus()

	// Create textarea for inline editing.
	editTa := textarea.New()
	editTa.Placeholder = "Edit your commit message..."
	editTa.ShowLineNumbers = false
	editTa.SetHeight(10)
	editTa.SetValue(message)
	editTa.Focus()

	return reviewModel{
		message:      message,
		diff:         diff,
		viewport:     vp,
		textarea:     ta,
		editTextarea: editTa,
	}
}

// Init initializes the model.
func (m reviewModel) Init() tea.Cmd {
	return textarea.Blink
}

// shouldShowDiff determines if the diff preview should be displayed.
func (m reviewModel) shouldShowDiff() bool {
	return m.height > 20 && len(m.diff) > 0
}

// Update handles messages and updates the model.
func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle inline edit mode.
		if m.editMode {
			switch msg.Type {
			case tea.KeyEsc:
				// Cancel edit mode without saving.
				m.editMode = false
				m.editTextarea.SetValue(m.message) // Reset to original
				return m, nil

			case tea.KeyCtrlC:
				// Cancel everything.
				m.action = ReviewReject
				m.done = true
				return m, tea.Quit

			default:
				// Check for Ctrl+Enter
				if msg.String() == "ctrl+enter" || (msg.Type == tea.KeyEnter && msg.Alt) {
					// Save the edited message.
					m.message = m.editTextarea.Value()
					m.editMode = false
					m.action = ReviewEditInline
					m.done = true
					return m, tea.Quit
				}
			}

			// Update edit textarea.
			m.editTextarea, cmd = m.editTextarea.Update(msg)
			return m, cmd
		}

		// Handle feedback mode.
		if m.showFeedback {
			switch msg.Type {
			case tea.KeyEsc:
				// Cancel feedback mode.
				m.showFeedback = false
				m.textarea.Reset()
				return m, nil

			case tea.KeyCtrlC:
				// Cancel everything.
				m.action = ReviewReject
				m.done = true
				return m, tea.Quit

			case tea.KeyEnter:
				if !m.textarea.Focused() {
					return m, nil
				}
				// Only submit on Ctrl+Enter or if not in textarea.
				if msg.Type == tea.KeyEnter && len(m.textarea.Value()) > 0 {
					m.feedback = m.textarea.Value()
					m.action = ReviewRegenerate
					m.done = true
					return m, tea.Quit
				}
			}

			// Update textarea.
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}

		// Handle review mode.
		switch msg.String() {
		case "y", "Y":
			m.action = ReviewAccept
			m.done = true
			return m, tea.Quit

		case "n", "N", "q", "Q":
			m.action = ReviewReject
			m.done = true
			return m, tea.Quit

		case "r", "R":
			m.showFeedback = true
			m.textarea.Focus()
			return m, textarea.Blink

		case "e", "E":
			// Edit using configured mode
			if m.preferExternal {
				// Use external editor
				m.action = ReviewEdit
				m.done = true
				return m, tea.Quit
			} else {
				// Use inline textarea editing
				m.editMode = true
				m.editTextarea.SetValue(m.message)
				m.editTextarea.Focus()
				return m, textarea.Blink
			}

		case "ctrl+c":
			m.action = ReviewReject
			m.done = true
			return m, tea.Quit

		// Viewport navigation.
		case "up", "k":
			m.viewport.LineUp(1)
		case "down", "j":
			m.viewport.LineDown(1)
		case "pgup":
			m.viewport.HalfViewUp()
		case "pgdown":
			m.viewport.HalfViewDown()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update textarea widths.
		m.textarea.SetWidth(msg.Width - 4)
		m.editTextarea.SetWidth(msg.Width - 4)

		// Calculate component heights
		headerHeight := lipgloss.Height(m.viewHeader())
		footerHeight := lipgloss.Height(m.viewFooter())
		messageBoxHeight := lipgloss.Height(messageBoxStyle.
			Width(msg.Width - 4).
			Render(m.message))

		// Only calculate and set viewport dimensions if we'll actually show it
		if m.shouldShowDiff() {
			// When diff is shown, we have these newlines:
			// - 1 after header
			// - 2 after message
			// - 1 after diff label
			// - 1 after viewport
			diffLabelHeight := 1
			newlines := 5

			reservedHeight := headerHeight + messageBoxHeight + diffLabelHeight + footerHeight + newlines

			// Fine-tuned adjustment to use all available space
			// Subtracting 3.5 effectively by adding 1 to viewport after calculation
			reservedHeight -= 3

			viewportHeight := max(msg.Height-reservedHeight, 3)

			// Add one more line to viewport to use the last remaining line
			viewportHeight += 1

			// Store for debug output
			m.debugReserved = reservedHeight
			m.debugViewportHeight = viewportHeight

			if !m.ready {
				m.viewport = viewport.New(msg.Width-2, viewportHeight)
				m.viewport.SetContent(formatDiff(m.diff, viewportHeight))
				m.ready = true
			} else {
				m.viewport.Width = msg.Width - 2
				m.viewport.Height = viewportHeight
				// Update content with new height to ensure padding
				m.viewport.SetContent(formatDiff(m.diff, viewportHeight))
			}
		} else if !m.ready {
			// Initialize a minimal viewport for potential later use
			// This won't be rendered but ensures m.ready is true
			m.viewport = viewport.New(msg.Width-2, 5)
			m.viewport.SetContent(formatDiff(m.diff, 5))
			m.ready = true
		}
	}

	// Update viewport.
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the model.
func (m reviewModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	// Show inline edit mode.
	if m.editMode {
		return m.viewEditMode()
	}

	// Show feedback mode.
	if m.showFeedback {
		return m.viewFeedback()
	}

	// Show review mode.
	return m.viewReview()
}

// viewReview renders the review screen.
func (m reviewModel) viewReview() string {
	var s strings.Builder

	// Header.
	s.WriteString(m.viewHeader())
	s.WriteString("\n")

	// Message box.
	messageBox := messageBoxStyle.
		Width(m.width - 4).
		Render(m.message)
	s.WriteString(messageBox)
	s.WriteString("\n\n")

	// Diff preview (if there's room).
	if m.shouldShowDiff() {
		s.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("Diff Preview (scroll with arrow keys):"))
		s.WriteString("\n")
		s.WriteString(m.viewport.View())
		s.WriteString("\n")
	}

	// Footer.
	s.WriteString(m.viewFooter())

	// Debug: Check if we're using all available height
	if m.height > 0 && os.Getenv("GAC_DEBUG") != "" {
		rendered := s.String()
		actualLines := strings.Count(rendered, "\n") + 1
		// Always show debug info when GAC_DEBUG is set
		unused := m.height - actualLines
		debugInfo := fmt.Sprintf("\n[DEBUG: Using %d/%d lines, %d unused | Reserved: %d, Viewport: %d]",
			actualLines, m.height, unused, m.debugReserved, m.debugViewportHeight)
		s.WriteString(debugInfo)

		// If there's still unused space, suggest adjustment
		if unused > 0 && unused < 10 {
			s.WriteString(fmt.Sprintf("\n[DEBUG: Suggest increasing viewport by %d lines]", unused))
		}
	}

	return s.String()
}

// viewFeedback renders the feedback input screen.
func (m reviewModel) viewFeedback() string {
	var s strings.Builder

	// Title.
	s.WriteString(feedbackStyle.Render("Provide feedback for regeneration:"))
	s.WriteString("\n\n")

	// Textarea.
	s.WriteString(focusedStyle.
		Width(m.width - 2).
		Render(m.textarea.View()))
	s.WriteString("\n\n")

	// Help.
	s.WriteString(helpStyle.Render("Press Enter to submit • Esc to cancel"))

	return s.String()
}

// viewEditMode renders the inline edit mode screen.
func (m reviewModel) viewEditMode() string {
	var s strings.Builder

	// Title.
	s.WriteString(titleStyle.Render("Edit Commit Message"))
	s.WriteString("\n\n")

	// Edit textarea with border.
	s.WriteString(focusedStyle.
		Width(m.width - 2).
		Render(m.editTextarea.View()))
	s.WriteString("\n\n")

	// Help text.
	s.WriteString(helpStyle.Render("Ctrl+Enter to save • Esc to cancel"))

	return s.String()
}

// viewHeader renders the header.
func (m reviewModel) viewHeader() string {
	return titleStyle.Render("Review Commit Message")
}

// viewFooter renders the footer with available actions.
func (m reviewModel) viewFooter() string {
	if m.showFeedback {
		return ""
	}

	// Define actions with their display text
	editText := "[e]dit - Edit message"
	if m.preferExternal {
		editText = "[e]dit - External editor"
	}

	actions := []struct {
		text string
		width int
	}{
		{"[y]es - Accept", 0},
		{"[n]o - Reject", 0},
		{"[r]egenerate - Provide feedback", 0},
		{editText, 0},
		{"[q]uit - Cancel", 0},
	}

	// Calculate width for each action
	for i := range actions {
		actions[i].width = lipgloss.Width(actions[i].text)
	}

	// If we have no width constraint or everything fits on one line, use single line
	separator := " • "
	separatorWidth := lipgloss.Width(separator)

	totalWidth := 0
	for i, action := range actions {
		totalWidth += action.width
		if i > 0 {
			totalWidth += separatorWidth
		}
	}

	// If everything fits on one line, return it
	if m.width <= 0 || totalWidth <= m.width-2 {
		items := make([]string, len(actions))
		for i, action := range actions {
			items[i] = action.text
		}
		return helpStyle.Render(strings.Join(items, separator))
	}

	// Dynamic line breaking - fit as many items as possible per line
	var lines []string
	var currentLine []string
	currentWidth := 0

	for _, action := range actions {
		// Check if adding this action would exceed the width
		testWidth := currentWidth
		if len(currentLine) > 0 {
			testWidth += separatorWidth
		}
		testWidth += action.width

		if testWidth > m.width-2 && len(currentLine) > 0 {
			// Start a new line
			lines = append(lines, strings.Join(currentLine, separator))
			currentLine = []string{action.text}
			currentWidth = action.width
		} else {
			// Add to current line
			if len(currentLine) > 0 {
				currentWidth += separatorWidth
			}
			currentWidth += action.width
			currentLine = append(currentLine, action.text)
		}
	}

	// Add the last line
	if len(currentLine) > 0 {
		lines = append(lines, strings.Join(currentLine, separator))
	}

	return helpStyle.Render(strings.Join(lines, "\n"))
}

// formatDiff truncates and formats the diff for display.
func formatDiff(diff string, minHeight int) string {
	lines := strings.Split(diff, "\n")
	maxLines := 50

	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, "... (diff truncated)")
	}

	// Apply basic coloring to diff lines.
	var formatted []string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "+"):
			formatted = append(formatted, lipgloss.NewStyle().
				Foreground(lipgloss.Color("42")).
				Render(line))
		case strings.HasPrefix(line, "-"):
			formatted = append(formatted, lipgloss.NewStyle().
				Foreground(lipgloss.Color("161")).
				Render(line))
		case strings.HasPrefix(line, "@@"):
			formatted = append(formatted, lipgloss.NewStyle().
				Foreground(lipgloss.Color("63")).
				Render(line))
		default:
			formatted = append(formatted, line)
		}
	}

	result := strings.Join(formatted, "\n")

	// Pad with empty lines if content is shorter than viewport height
	if minHeight > 0 {
		lineCount := len(formatted)
		if lineCount < minHeight {
			// Add empty lines to fill the viewport
			for i := lineCount; i < minHeight; i++ {
				result += "\n"
			}
		}
	}

	return result
}

// ShowCommitReview displays the interactive commit review screen.
// Returns the action taken, feedback/edited message, and any error.
func ShowCommitReview(message, diff, editorMode string) (ReviewAction, string, error) {
	m := newReviewModel(message, diff)

	// If editor mode is set to external, swap the key bindings
	if editorMode == "external" {
		m.preferExternal = true
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return ReviewReject, "", fmt.Errorf("failed to run review UI: %w", err)
	}

	reviewModel := finalModel.(reviewModel)

	// For inline edit, return the edited message; otherwise return feedback
	if reviewModel.action == ReviewEditInline {
		return reviewModel.action, reviewModel.message, nil
	}
	return reviewModel.action, reviewModel.feedback, nil
}