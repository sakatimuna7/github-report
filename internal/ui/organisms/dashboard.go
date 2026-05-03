package organisms

import (
	"fmt"
	"sort"
	"strings"

	"github-report-ai/internal/ui/atoms"
	"github-report-ai/pkg/github"
	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
)

func RenderDashboard(data github.DashboardData) string {
	// 1. Language Stack
	type lang struct {
		Name  string
		Bytes int
	}
	var langs []lang
	totalBytes := 0
	for n, b := range data.Languages {
		langs = append(langs, lang{n, b})
		totalBytes += b
	}
	sort.Slice(langs, func(i, j int) bool { return langs[i].Bytes > langs[j].Bytes })

	// Limit to top 8
	if len(langs) > 8 {
		langs = langs[:8]
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(1, 2).
		MarginBottom(1).
		Width(80)

	// Language Bar
	barWidth := 74
	bar := ""
	colors := []string{"#3178c6", "#f1e05a", "#e34c26", "#563d7c", "#b07219", "#41b883", "#00b4ab", "#89e051"}
	
	legend := ""
	for i, l := range langs {
		pct := float64(l.Bytes) / float64(totalBytes)
		w := int(pct * float64(barWidth))
		if w < 1 && pct > 0 { w = 1 }
		
		c := colors[i%len(colors)]
		char := "█"
		
		bar += lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(strings.Repeat(char, w))
		
		dot := lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render("●")
		legend += fmt.Sprintf("%s %s %s   ", dot, l.Name, color.HiBlackString("%.1f%%", pct*100))
		if (i+1)%4 == 0 { legend += "\n" }
	}

	stackView := lipgloss.JoinVertical(lipgloss.Left,
		atoms.TitleStyle.Render(" Global Project Stack"),
		bar,
		"\n"+legend,
	)

	// 2. Contributions Chart
	maxContrib := 1
	for _, v := range data.Contributions {
		if v > maxContrib { maxContrib = v }
	}

	chart := ""
	chartHeight := 5
	for h := chartHeight; h > 0; h-- {
		labelVal := float64(maxContrib) * float64(h) / float64(chartHeight)
		line := color.HiBlackString("%2.0f ┤ ", labelVal)
		for _, v := range data.Contributions {
			level := int(float64(v) / float64(maxContrib) * float64(chartHeight))
			if v > 0 && level == 0 { level = 1 } // Ensure visibility
			if level >= h {
				line += lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("┃")
			} else {
				line += color.HiBlackString(" ")
			}
		}
		chart += line + "\n"
	}
	chart += color.HiBlackString("   └" + strings.Repeat("─", 30)) + "\n"
	chart += color.HiBlackString("    30 Days Contribution Activity")

	contribView := lipgloss.JoinVertical(lipgloss.Left,
		atoms.TitleStyle.Render("📊 Contributions"),
		chart,
	)

	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, stackView, "\n", contribView))
}
