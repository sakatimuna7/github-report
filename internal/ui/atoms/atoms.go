package atoms

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	DocStyle = lipgloss.NewStyle().Margin(1, 2)
	
	CyanStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
	GrayStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	TitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true).MarginLeft(2).MarginBottom(1)
	
	NoticeStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42")).
			Padding(0, 1).
			MarginLeft(2)
)

func GetBanner() string {
	banner := `   ____ _ _   _   _       _     ____                       _   
  / ___(_) |_| | | |_   _| |__ |  _ \ ___ _ __   ___  _ __| |_ 
 | |  _| | __| |_| | | | | '_ \| |_) / _ \ '_ \ / _ \| '__| __|
 | |_| | | |_|  _  | |_| | |_) |  _ <  __/ |_) | (_) | |  | |_ 
  \____|_|\__|_| |_|\__,_|_.__/|_| \_\___| .__/ \___/|_|   \__|
                                         |_|                   `

	return "\n" + CyanStyle.Render(banner) + "\n" +
		GrayStyle.Render("   AI-Powered GitHub Commit Summarizer") + "\n" +
		GrayStyle.Render("   ===================================") + "\n"
}

func Sh(c string, a ...string) string {
	// Utility kept here or in a separate utils package, but atoms need it for some dynamic labels?
	// Actually better in a utils package. Let's put it in internal/utils if needed.
	return "" 
}
