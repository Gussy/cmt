package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SecretAction represents the user's choice for handling detected secrets.
type SecretAction int

const (
	// ActionAbort cancels the commit entirely.
	ActionAbort SecretAction = iota
	// ActionUnstage removes files with secrets from staging.
	ActionUnstage
	// ActionContinue proceeds despite the warnings.
	ActionContinue
)

// Secret represents a detected secret in the code.
type Secret struct {
	Type     string // Type of secret (e.g., "AWS Access Key").
	FilePath string // File containing the secret.
	Line     int    // Line number where the secret was found.
	Match    string // Redacted preview of the secret.
}

// secretWarningModel is the Bubble Tea model for the secret warning screen.
type secretWarningModel struct {
	secrets []Secret
	action  SecretAction
	done    bool
	width   int
	height  int
}

var (
	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")).
			MarginBottom(1)

	secretTypeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214"))

	filePathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("63"))

	redactedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("239"))

	dangerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196")).
			Padding(1)

	actionStyle = lipgloss.NewStyle().
			MarginTop(1).
			Foreground(lipgloss.Color("241"))
)

// newSecretWarningModel creates a new secret warning model.
func newSecretWarningModel(secrets []Secret) secretWarningModel {
	return secretWarningModel{
		secrets: secrets,
	}
}

// Init initializes the model.
func (m secretWarningModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model.
func (m secretWarningModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "a", "A":
			m.action = ActionAbort
			m.done = true
			return m, tea.Quit

		case "u", "U":
			m.action = ActionUnstage
			m.done = true
			return m, tea.Quit

		case "c", "C":
			m.action = ActionContinue
			m.done = true
			return m, tea.Quit

		case "ctrl+c", "q", "Q":
			m.action = ActionAbort
			m.done = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// View renders the secret warning screen.
func (m secretWarningModel) View() string {
	var s strings.Builder

	// Warning header.
	s.WriteString(warningStyle.Render("⚠️  Secrets Detected!"))
	s.WriteString("\n\n")

	// Explanation.
	s.WriteString("The following potential secrets were found in your staged files:\n\n")

	// List secrets.
	for _, secret := range m.secrets {
		secretInfo := fmt.Sprintf(
			"%s: %s (%s:%d)",
			secretTypeStyle.Render(secret.Type),
			redactedStyle.Render(secret.Match),
			filePathStyle.Render(secret.FilePath),
			secret.Line,
		)
		s.WriteString("  • " + secretInfo + "\n")
	}

	s.WriteString("\n")

	// Danger warning.
	if m.width > 60 {
		dangerText := "⚠️  DANGER: Committing secrets can expose sensitive data!\n" +
			"Consider using environment variables or a secrets management system instead."
		s.WriteString(dangerBoxStyle.
			Width(m.width - 4).
			Render(dangerText))
		s.WriteString("\n\n")
	}

	// Actions.
	s.WriteString("What would you like to do?\n\n")
	s.WriteString(actionStyle.Render(
		"[a]bort - Cancel the commit\n" +
			"[u]nstage - Remove these files from staging\n" +
			"[c]ontinue - Proceed anyway (NOT RECOMMENDED)\n\n" +
			"Press [q] to quit",
	))

	return s.String()
}

// ShowSecretWarning displays the secret warning screen.
func ShowSecretWarning(secrets []Secret) (SecretAction, error) {
	if len(secrets) == 0 {
		return ActionContinue, nil
	}

	m := newSecretWarningModel(secrets)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return ActionAbort, fmt.Errorf("failed to run secret warning UI: %w", err)
	}

	warningModel := finalModel.(secretWarningModel)
	return warningModel.action, nil
}

// FormatSecretForDisplay redacts a secret for safe display.
func FormatSecretForDisplay(secret string) string {
	if len(secret) <= 8 {
		return strings.Repeat("*", len(secret))
	}

	// Show first 4 and last 3 characters.
	return secret[:4] + "..." + secret[len(secret)-3:]
}

// MockSecrets returns mock secrets for testing the UI.
func MockSecrets() []Secret {
	return []Secret{
		{
			Type:     "AWS Access Key",
			FilePath: "config/aws.json",
			Line:     15,
			Match:    "AKIA...xyz",
		},
		{
			Type:     "GitHub Token",
			FilePath: "scripts/deploy.sh",
			Line:     8,
			Match:    "ghp_...abc",
		},
		{
			Type:     "Private Key",
			FilePath: ".env",
			Line:     3,
			Match:    "-----BEGIN RSA...KEY-----",
		},
	}
}