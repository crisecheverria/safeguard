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
	list          list.Model
	selectedFiles []string
	quitting      bool
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
		key.WithHelp("enter", "confirm selection"),
	),
	quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q/ctrl+c", "quit"),
	),
	filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
}

// Item implementation for list
type fileItem struct {
	path     string
	selected bool
}

func (i fileItem) Title() string { 
	if i.selected {
		return "[✓] " + i.path
	}
	return "[ ] " + i.path
}
func (i fileItem) Description() string { 
	if i.selected {
		return "Selected for analysis"
	}
	return "" 
}
func (i fileItem) FilterValue() string { return i.path }

// File selector initialization
func launchFileSelector() ([]string, error) {
	// Get list of git files
	files, err := listGitFiles()
	if err != nil {
		return nil, err
	}

	// Convert strings to list items
	items := make([]list.Item, 0, len(files))
	for _, file := range files {
		if file != "" { // Skip empty lines
			items = append(items, fileItem{path: file, selected: false})
		}
	}

	// Create the list model with proper sizing
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#0000FF"))
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	
	l := list.New(items, delegate, 80, 20) // Set explicit width and height
	l.Title = "Select files to analyze (Space to toggle, Enter to confirm)"
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
			key.NewBinding(
				key.WithKeys("space"),
				key.WithHelp("space", "toggle selection"),
			),
		}
	}

	// Create the model
	model := fileModel{list: l, selectedFiles: []string{}}

	// Run the program
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m, err := p.Run()
	if err != nil {
		return nil, err
	}

	// Get the selected files
	finalModel, ok := m.(fileModel)
	if !ok || len(finalModel.selectedFiles) == 0 {
		return nil, fmt.Errorf("no files selected")
	}

	return finalModel.selectedFiles, nil
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
	return tea.EnterAltScreen
}

func (m fileModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Update list size when terminal is resized
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 6) // Leave space for selected files display
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, keys.select_):
			// Confirm selection and quit
			return m, tea.Quit

		case msg.String() == " ": // Space key to toggle selection
			// Toggle selection of current item
			if i, ok := m.list.SelectedItem().(fileItem); ok {
				// Update the item in the list
				newItem := fileItem{path: i.path, selected: !i.selected}
				
				// Update the selected files list
				if newItem.selected {
					// Add to selected files
					m.selectedFiles = append(m.selectedFiles, i.path)
				} else {
					// Remove from selected files
					for idx, file := range m.selectedFiles {
						if file == i.path {
							m.selectedFiles = append(m.selectedFiles[:idx], m.selectedFiles[idx+1:]...)
							break
						}
					}
				}
				
				// Update the item in the list
				items := m.list.Items()
				currentIndex := m.list.Index()
				if currentIndex < len(items) {
					items[currentIndex] = newItem
					m.list.SetItems(items)
				}
			}
			return m, nil
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

	// Show selected files count in a more compact way
	selectedInfo := fmt.Sprintf("Selected: %d files", len(m.selectedFiles))
	if len(m.selectedFiles) > 0 && len(m.selectedFiles) <= 3 {
		selectedInfo += " (" + strings.Join(m.selectedFiles, ", ") + ")"
	} else if len(m.selectedFiles) > 3 {
		selectedInfo += " (" + strings.Join(m.selectedFiles[:3], ", ") + "...)"
	}
	
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00")).
		Bold(true)

	return m.list.View() + "\n" + selectedStyle.Render(selectedInfo)
}