package tui

import (
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"sonar-local/internal/ops"
)

// RunScan streams a scanner run into a scrollable viewport. The caller owns the token
// lifecycle (mint before, revoke after).
func RunScan(o *ops.Ops, args []string, title, dashURL string) error {
	cmd, lines, err := o.Rt.RunStream(args...)
	if err != nil {
		return err
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	m := scanModel{spinner: sp, lines: lines, cmd: cmd, title: title, dashURL: dashURL}

	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	if sm, ok := final.(scanModel); ok {
		return sm.err
	}
	return nil
}

type lineMsg string
type streamClosedMsg struct{}
type scanEndMsg struct{ err error }

type scanModel struct {
	viewport viewport.Model
	spinner  spinner.Model
	lines    <-chan string
	cmd      *exec.Cmd
	content  []string
	ready    bool
	done     bool
	err      error
	title    string
	dashURL  string
}

func waitLine(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return streamClosedMsg{}
		}
		return lineMsg(line)
	}
}

func waitProc(cmd *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		return scanEndMsg{err: cmd.Wait()}
	}
}

func (m scanModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitLine(m.lines))
}

func (m scanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.cmd != nil && m.cmd.Process != nil {
				_ = m.cmd.Process.Kill()
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h := msg.Height - 4
		if h < 3 {
			h = 3
		}
		if !m.ready {
			m.viewport = viewport.New(msg.Width, h)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = h
		}
		m.viewport.SetContent(strings.Join(m.content, "\n"))
		m.viewport.GotoBottom()
		return m, nil
	case lineMsg:
		m.content = append(m.content, string(msg))
		if m.ready {
			m.viewport.SetContent(strings.Join(m.content, "\n"))
			m.viewport.GotoBottom()
		}
		return m, waitLine(m.lines)
	case streamClosedMsg:
		return m, waitProc(m.cmd)
	case scanEndMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m scanModel) View() string {
	header := m.title
	if !m.done {
		header = m.spinner.View() + " " + m.title
	}

	body := strings.Join(m.content, "\n")
	if m.ready {
		body = m.viewport.View()
	}

	footer := dimStyle.Render("↑/↓ scroll · ctrl+c quit")
	if m.done {
		if m.err != nil {
			footer = errStyle.Render("✗ scan failed: " + m.err.Error())
		} else {
			footer = successStyle.Render("✓ done") + " · " + m.dashURL
		}
	}

	return header + "\n" + body + "\n" + footer + "\n"
}
