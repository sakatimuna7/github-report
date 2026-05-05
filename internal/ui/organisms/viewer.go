package organisms

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ReportViewerModel struct {
	Viewport viewport.Model
	Ready    bool
	Content  string
	Action   string
	Toast    string
}

func (m ReportViewerModel) Init() tea.Cmd {
	return nil
}

func (m ReportViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch k := msg.String(); k {
		case "ctrl+c", "q", "esc":
			m.Action = "quit"
			return m, tea.Quit
		case "r":
			m.Action = "regen"
			return m, tea.Quit
		case "p":
			m.Action = "print"
			return m, tea.Quit
		case "e":
			m.Action = "export_sheets"
			return m, tea.Quit
		case "t":
			m.Action = "edit"
			return m, tea.Quit
		case "c":
			c := exec.Command("pbcopy")
			c.Stdin = strings.NewReader(m.Content)
			if err := c.Run(); err == nil {
				m.Toast = "✅ Copied"
			} else {
				m.Toast = "❌ Copy Failed"
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		headerHeight := 3
		footerHeight := 3
		verticalMarginHeight := headerHeight + footerHeight

		if !m.Ready {
			m.Viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.Viewport.SetContent(m.Content)
			m.Ready = true
		} else {
			m.Viewport.Width = msg.Width
			m.Viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	m.Viewport, cmd = m.Viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m ReportViewerModel) View() string {
	if !m.Ready {
		return "\n  Initializing..."
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("42")).
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(lipgloss.Color("42")).
		Padding(0, 1).
		Width(m.Viewport.Width)

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(lipgloss.Color("241")).
		Padding(0, 1).
		Width(m.Viewport.Width)

	header := headerStyle.Render("✨ GITHUB REPORT GENERATED")
	
	footerText := fmt.Sprintf("%3.f%% • [c] copy • [t] edit • [p] print • [e] export sheets • [r] regen • [q] quit", m.Viewport.ScrollPercent()*100)
	if m.Toast != "" {
		footerText = fmt.Sprintf("%3.f%% • %s • [c] copy • [t] edit • [p] print • [e] export • [r] regen • [q] quit", m.Viewport.ScrollPercent()*100, m.Toast)
	}
	footer := footerStyle.Render(footerText)

	return fmt.Sprintf("%s\n%s\n%s", header, m.Viewport.View(), footer)
}
