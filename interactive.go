package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model for file selection
type fileModel struct {
	list     list.Model
	selected string
	quitting bool
}

// Define keymaps for the file selector
type keyMap struct {
	select_ key.Binding
	quit    key.Binding
	filter  key.Binding
}

var keys = keyMap{
	select_: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q/ctrl+c", "quit"),
	),
	filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter files (type to filter)"),
	),
}

// Item implementation for list
type fileItem struct {
	path string
}

func (i fileItem) Title() string       { return i.path }
func (i fileItem) Description() string { return "" }
func (i fileItem) FilterValue() string { return i.path }

// File selector initialization
func launchFileSelector() (string, error) {
	// Get list of git files
	files, err := listGitFiles()
	if err != nil {
		return "", err
	}

	// Convert strings to list items
	items := make([]list.Item, 0, len(files))
	for _, file := range files {
		if file != "" { // Skip empty lines
			items = append(items, fileItem{path: file})
		}
	}

	// Create the list model
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select a file to analyze"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#0000FF")).Bold(true).Padding(0, 1)
	l.SetShowHelp(true)
	
	// Help text for filtering
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("/"),
				key.WithHelp("/", "filter"),
			),
		}
	}

	// Create the model
	model := fileModel{list: l}

	// Run the program
	p := tea.NewProgram(model, tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return "", err
	}

	// Get the selected file
	finalModel, ok := m.(fileModel)
	if !ok || finalModel.selected == "" {
		return "", fmt.Errorf("no file selected")
	}

	return finalModel.selected, nil
}

// List all files tracked by git
func listGitFiles() ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list git files: %w", err)
	}

	files := strings.Split(string(output), "\n")
	return files, nil
}

// BubbleTea Model implementation
func (m fileModel) Init() tea.Cmd {
	return nil
}

func (m fileModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, keys.select_):
			i, ok := m.list.SelectedItem().(fileItem)
			if ok {
				m.selected = i.path
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m fileModel) View() string {
	if m.quitting {
		return "Bye!\n"
	}

	return "\n" + m.list.View() + "\n"
}