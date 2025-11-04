package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EditInEditor opens the system editor for the user to edit the commit message.
func EditInEditor(message string) (string, error) {
	// Determine which editor to use.
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Try common editors in order of preference.
		editors := []string{"vim", "vi", "nano", "emacs", "code", "subl"}
		for _, e := range editors {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}

	if editor == "" {
		return "", fmt.Errorf("no editor found. Please set $EDITOR environment variable")
	}

	// Create a temporary file for editing.
	tmpFile, err := os.CreateTemp("", "cmt-commit-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write the current message to the file.
	if _, err := tmpFile.WriteString(message); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Add help comments (like git does).
	helpText := `

# Please enter the commit message for your changes. Lines starting
# with '#' will be ignored, and an empty message aborts the commit.
#
# You can use the conventional commit format:
#   feat: add new feature
#   fix: fix a bug
#   docs: update documentation
#   style: formatting changes
#   refactor: code refactoring
#   test: add tests
#   chore: maintenance tasks
#`
	if _, err := tmpFile.WriteString(helpText); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write help text: %w", err)
	}

	tmpFile.Close()

	// Open the editor.
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run editor: %w", err)
	}

	// Read the edited content.
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read edited file: %w", err)
	}

	// Process the content (remove comments and trim).
	editedMessage := processEditedMessage(string(content))

	if editedMessage == "" {
		return "", fmt.Errorf("commit message cannot be empty")
	}

	return editedMessage, nil
}

// processEditedMessage removes comment lines and trims the message.
func processEditedMessage(content string) string {
	lines := strings.Split(content, "\n")
	var processedLines []string

	for _, line := range lines {
		// Skip comment lines.
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		processedLines = append(processedLines, line)
	}

	// Join and trim the final message.
	result := strings.Join(processedLines, "\n")
	result = strings.TrimSpace(result)

	return result
}

// GetEditorName returns the name of the editor that will be used.
func GetEditorName() string {
	editor := os.Getenv("EDITOR")
	if editor != "" {
		// Extract just the program name from the path.
		return filepath.Base(editor)
	}

	// Check for common editors.
	editors := []string{"vim", "vi", "nano", "emacs", "code", "subl"}
	for _, e := range editors {
		if _, err := exec.LookPath(e); err == nil {
			return e
		}
	}

	return "default editor"
}