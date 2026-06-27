package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"sonar-local/internal/ops"
)

// RunUp drives the startup sequence with a progress bar and a spinner.
func RunUp(o *ops.Ops) error {
	steps := ops.DefaultUpSteps(o)
	final := successStyle.Render("✓ SonarQube is ready") + "\n" +
		"  UI:    " + o.Cfg.LocalURL() + "\n" +
		"  Login: admin / $SONAR_ADMIN_PASSWORD\n"

	m, err := tea.NewProgram(newUpModel(steps, final)).Run()
	if err != nil {
		return err
	}
	if um, ok := m.(upModel); ok {
		return um.err
	}
	return nil
}

type stepDoneMsg struct {
	index int
	err   error
}

type upModel struct {
	steps    []ops.Step
	index    int
	spinner  spinner.Model
	progress progress.Model
	done     bool
	err      error
	finalMsg string
}

func newUpModel(steps []ops.Step, finalMsg string) upModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return upModel{
		steps:    steps,
		spinner:  sp,
		progress: progress.New(progress.WithDefaultGradient()),
		finalMsg: finalMsg,
	}
}

func runStepCmd(steps []ops.Step, i int) tea.Cmd {
	return func() tea.Msg {
		return stepDoneMsg{index: i, err: steps[i].Run()}
	}
}

func (m upModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, runStepCmd(m.steps, 0))
}

func (m upModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if s := msg.String(); s == "ctrl+c" || s == "q" {
			return m, tea.Quit
		}
	case stepDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.index = msg.index + 1
		if m.index >= len(m.steps) {
			m.done = true
			return m, tea.Quit
		}
		pct := float64(m.index) / float64(len(m.steps))
		return m, tea.Batch(runStepCmd(m.steps, m.index), m.progress.SetPercent(pct))
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		pm, cmd := m.progress.Update(msg)
		m.progress = pm.(progress.Model)
		return m, cmd
	}
	return m, nil
}

func (m upModel) View() string {
	if m.err != nil {
		return errStyle.Render("✗ "+m.err.Error()) + "\n"
	}
	if m.done {
		return m.finalMsg
	}
	label := ""
	if m.index < len(m.steps) {
		label = m.steps[m.index].Label
	}
	return fmt.Sprintf("%s %s\n\n%s\n", m.spinner.View(), label, m.progress.View())
}
