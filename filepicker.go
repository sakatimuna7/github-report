package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
)

type filePickerModel struct {
	filepicker   filepicker.Model
	selectedFile string
	quitting     bool
	err          error
}

func (m filePickerModel) Init() tea.Cmd {
	return m.filepicker.Init()
}

func (m filePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.filepicker, cmd = m.filepicker.Update(msg)

	// Did the user select a file?
	if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
		// Get the path of the selected file.
		m.selectedFile = path
		m.quitting = true
		return m, tea.Quit
	}

	// Did the user select a disabled file?
	// This is only necessary if you've added disabled files using
	// filepicker.FileAllowed (see below).
	if didSelect, path := m.filepicker.DidSelectDisabledFile(msg); didSelect {
		// Let's clear the selectedFile and display an error.
		m.err = errors.New(path + " is not valid.")
		m.selectedFile = ""
		return m, tea.Batch(cmd, tea.Printf("The file %s is disabled.", path))
	}

	return m, cmd
}

func (m filePickerModel) View() string {
	if m.quitting {
		return ""
	}
	var s strings.Builder
	s.WriteString("\n  Select your credentials.json file:\n\n")
	if m.err != nil {
		s.WriteString(m.filepicker.Styles.DisabledFile.Render(m.err.Error()) + "\n")
	} else {
		s.WriteString("  Pick a file or press 'esc' to cancel.\n\n")
	}
	s.WriteString(m.filepicker.View() + "\n")
	return s.String()
}

func runFilePicker() string {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".json"}
	fp.CurrentDirectory, _ = os.UserHomeDir()

	m := filePickerModel{
		filepicker: fp,
	}
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		fmt.Println("Error running file picker:", err)
		return ""
	}
	fm := finalModel.(filePickerModel)
	if fm.selectedFile != "" {
		return fm.selectedFile
	}
	return ""
}
