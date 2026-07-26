package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true)

	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type pickModel struct {
	selected  string
	choices   []string
	cursor    int
	cancelled bool
}

func (m pickModel) Init() tea.Cmd { return nil }

func (m pickModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "ctrl+c", "q":
		m.cancelled = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.choices)-1 {
			m.cursor++
		}
	case "enter":
		m.selected = m.choices[m.cursor]
		return m, tea.Quit
	}

	return m, nil
}

func (m pickModel) View() string {
	s := "\n  " + titleStyle.Render("Select profile:") + "\n\n"

	for i, choice := range m.choices {
		if i == m.cursor {
			s += fmt.Sprintf("  %s %s\n", cursorStyle.Render("▸"), choice)
		} else {
			s += fmt.Sprintf("    %s\n", choice)
		}
	}

	if m.selected != "" {
		s += "\n  " + selectedStyle.Render("✓ "+m.selected)
	}

	s += "\n  " + hintStyle.Render("↑/↓ navigate  ↵ select  q cancel") + "\n"
	return s
}

func pickProfileBubbletea(names []string) (string, error) {
	p := tea.NewProgram(pickModel{choices: names})
	m, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("TUI error: %w", err)
	}

	model := m.(pickModel)
	if model.cancelled {
		return "", fmt.Errorf("selection cancelled")
	}
	return model.selected, nil
}
