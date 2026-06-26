package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// RunMenu shows an interactive list and returns the chosen command ("up", "scan",
// "status", "down") or "" if the user quit.
func RunMenu() (string, error) {
	items := []list.Item{
		menuItem{name: "up", desc: "Start the local SonarQube stack"},
		menuItem{name: "scan", desc: "Scan the current directory"},
		menuItem{name: "status", desc: "Show stack status"},
		menuItem{name: "down", desc: "Stop the stack"},
	}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Local SonarQube"

	final, err := tea.NewProgram(menuModel{list: l}, tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	if mm, ok := final.(menuModel); ok {
		return mm.choice, nil
	}
	return "", nil
}

type menuItem struct {
	name string
	desc string
}

func (i menuItem) Title() string       { return i.name }
func (i menuItem) Description() string  { return i.desc }
func (i menuItem) FilterValue() string { return i.name }

type menuModel struct {
	list   list.Model
	choice string
}

func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if it, ok := m.list.SelectedItem().(menuItem); ok {
				m.choice = it.name
			}
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m menuModel) View() string { return m.list.View() }
