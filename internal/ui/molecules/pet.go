package molecules

import (
	"fmt"

	"github-report-ai/pkg/github"
	"github.com/charmbracelet/lipgloss"
)

func RenderPet(stats github.CommitStats) string {
	var body string
	var mood string
	var color lipgloss.Color

	// Mood logic based on stats
	if stats.Overtime > stats.Total/3 && stats.Total > 0 {
		mood = "Ngantuk / Sleepy (Zzz)"
		body = "  (-__-)zzZ\n  (  >  )>\n   /    \\"
		color = lipgloss.Color("33") // Blue
	} else if stats.Fixes > stats.Features && stats.Total > 0 {
		mood = "Pusing / Dizzy (@_@)"
		body = "  (@__@)\n  (  >  )>\n   /    \\"
		color = lipgloss.Color("196") // Red
	} else if stats.Features > 5 {
		mood = "Semangat / Productive (On Fire!)"
		body = "  (^__^)\n <(  >  )>\n   /    \\"
		color = lipgloss.Color("42") // Green
	} else if stats.Total > 20 {
		mood = "Gahar / Buff (Absolute Unit)"
		body = "  (•̀_•́)\n <(  V  )>\n   /    \\"
		color = lipgloss.Color("214") // Gold
	} else {
		mood = "Santai / Chillin'"
		body = "  (•‿•)\n  (  >  )>\n   /    \\"
		color = lipgloss.Color("99") // Purple
	}

	style := lipgloss.NewStyle().Foreground(color).Bold(true)
	
	container := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 2).
		Width(30).
		Align(lipgloss.Center)

	content := fmt.Sprintf("%s\n\n%s", style.Render(body), lipgloss.NewStyle().Faint(true).Render("Status: "+mood))
	
	return container.Render(content)
}
