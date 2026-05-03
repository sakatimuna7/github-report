package pages

import (
	"fmt"

	"github-report-ai/internal/ui/atoms"
	"github-report-ai/internal/ui/molecules"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/creativeprojects/go-selfupdate"
)

type MenuModel struct {
	List      list.Model
	Choice    string
	Quitting  bool
	LatestRel *selfupdate.Release
}

func (m MenuModel) Init() tea.Cmd {
	return nil
}

func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q", "esc":
			m.Quitting = true
			return m, tea.Quit
		case "enter":
			i, ok := m.List.SelectedItem().(molecules.MenuItem)
			if ok {
				m.Choice = i.ActionStr
			}
			return m, tea.Quit
		case "u":
			if m.LatestRel != nil {
				m.Choice = "Update"
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		h, v := atoms.DocStyle.GetFrameSize()
		bannerHeight := 10
		if m.LatestRel != nil {
			bannerHeight += 4
		}
		m.List.SetSize(msg.Width-h, msg.Height-v-bannerHeight)
	}

	var cmd tea.Cmd
	m.List, cmd = m.List.Update(msg)
	return m, cmd
}

func (m MenuModel) View() string {
	if m.Quitting || m.Choice != "" {
		return ""
	}
	
	view := atoms.GetBanner()
	if m.LatestRel != nil {
		notice := fmt.Sprintf("🎉 Update Available: %s\nPress 'u' to update now", m.LatestRel.Version())
		view += atoms.NoticeStyle.Render(notice) + "\n"
	}
	
	return view + "\n" + atoms.DocStyle.Render(m.List.View())
}
