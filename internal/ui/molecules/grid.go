package molecules

import (
	"fmt"
	"strings"

	"github-report-ai/pkg/github"
	"github.com/charmbracelet/lipgloss"
)

func RenderContributionMap(stats github.CommitStats) string {
	if stats.Total == 0 {
		return ""
	}

	featColor := lipgloss.Color("42")   // Green
	fixColor := lipgloss.Color("196")  // Red
	refColor := lipgloss.Color("33")   // Blue
	docColor := lipgloss.Color("214")  // Orange/Yellow
	otherColor := lipgloss.Color("244") // Gray

	featStyle := lipgloss.NewStyle().Foreground(featColor)
	fixStyle := lipgloss.NewStyle().Foreground(fixColor)
	refStyle := lipgloss.NewStyle().Foreground(refColor)
	docStyle := lipgloss.NewStyle().Foreground(docColor)
	otherStyle := lipgloss.NewStyle().Foreground(otherColor)

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("📊 Contribution Map (Commit Types)") + "\n\n")

	// Create a visual grid of blocks (■)
	blocks := []string{}
	for i := 0; i < stats.Features; i++ { blocks = append(blocks, featStyle.Render("■")) }
	for i := 0; i < stats.Fixes; i++ { blocks = append(blocks, fixStyle.Render("■")) }
	for i := 0; i < stats.Refactors; i++ { blocks = append(blocks, refStyle.Render("■")) }
	for i := 0; i < stats.Docs; i++ { blocks = append(blocks, docStyle.Render("■")) }
	for i := 0; i < stats.Others; i++ { blocks = append(blocks, otherStyle.Render("■")) }

	// Group blocks in rows of 20
	for i, b := range blocks {
		sb.WriteString(b + " ")
		if (i+1)%20 == 0 {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n\n")

	// Legend
	sb.WriteString(fmt.Sprintf("%s Feat (%d)  ", featStyle.Render("■"), stats.Features))
	sb.WriteString(fmt.Sprintf("%s Fix (%d)  ", fixStyle.Render("■"), stats.Fixes))
	sb.WriteString(fmt.Sprintf("%s Refactor (%d)  ", refStyle.Render("■"), stats.Refactors))
	sb.WriteString(fmt.Sprintf("%s Docs (%d)  ", docStyle.Render("■"), stats.Docs))
	sb.WriteString(fmt.Sprintf("%s Other (%d)\n", otherStyle.Render("■"), stats.Others))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Render(sb.String())
}
