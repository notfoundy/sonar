package tui

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"sonar-local/internal/ops"
)

// RunStatus renders the container states in a table plus the live server status.
func RunStatus(o *ops.Ops) error {
	rows := []table.Row{
		{o.Cfg.ServerContainer, o.ContainerState(o.Cfg.ServerContainer)},
		{o.Cfg.DBContainer, o.ContainerState(o.Cfg.DBContainer)},
	}
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Container", Width: 18},
			{Title: "State", Width: 12},
		}),
		table.WithRows(rows),
		table.WithHeight(len(rows)+1),
	)

	m := statusModel{table: t, server: o.CurrentStatus(), url: o.Cfg.LocalURL()}
	_, err := tea.NewProgram(m).Run()
	return err
}

type statusModel struct {
	table  table.Model
	server string
	url    string
}

func (m statusModel) Init() tea.Cmd { return nil }

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "q", "enter", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m statusModel) View() string {
	statusLine := warnStyle.Render("Server: "+m.server) + "  " + m.url
	if m.server == "UP" {
		statusLine = successStyle.Render("Server: UP") + "  " + m.url
	}
	return titleStyle.Render("SonarQube status") + "\n\n" +
		m.table.View() + "\n\n" +
		statusLine + "\n" +
		dimStyle.Render("press q to quit") + "\n"
}
