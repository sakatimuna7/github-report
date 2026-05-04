package main

import (
	"encoding/base64"
	"fmt"
	"os"

	"github-report-ai/internal/config"
	"github-report-ai/internal/report"
	"github-report-ai/internal/ui/molecules"
	"github-report-ai/internal/ui/pages"
	"github-report-ai/internal/updater"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/joho/godotenv"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
	EncodedGeminiKey = "" // Injected at build time
	EncodedGoogleCredentials = "" // Injected at build time (JSON content)
	latestRelease *selfupdate.Release
)

func printVersion() {
	fmt.Printf("ghreport version %s\n", Version)
	fmt.Printf("commit: %s\n", Commit)
	fmt.Printf("built at: %s\n", BuildTime)
}

func main() {
	var isCI bool
	for _, arg := range os.Args {
		if arg == "-v" || arg == "--version" || arg == "version" {
			printVersion()
			return
		}
		if arg == "--ci" {
			isCI = true
		}
	}

	latestRelease = updater.CheckForUpdates(Version)

	_ = godotenv.Load()
	h, _ := os.UserHomeDir()
	confPath := h + "/.ghreport"
	if h != "" {
		_ = config.LoadEnv(confPath)
	}

	// Fallback to embedded Gemini key if not set in environment
	if os.Getenv("GEMINI_API_KEY") == "" && EncodedGeminiKey != "" {
		decoded, err := base64.StdEncoding.DecodeString(EncodedGeminiKey)
		if err == nil {
			os.Setenv("GEMINI_API_KEY", string(decoded))
		}
	}

	// Fallback to embedded Google Credentials if not set in environment
	if os.Getenv("GOOGLE_CREDENTIALS_JSON") == "" && EncodedGoogleCredentials != "" {
		decoded, err := base64.StdEncoding.DecodeString(EncodedGoogleCredentials)
		if err == nil {
			os.Setenv("GOOGLE_CREDENTIALS_JSON", string(decoded))
		}
	}

	if isCI {
		report.Run(confPath)
		return
	}

	items := []list.Item{
		molecules.NewMenuItem("🚀 Report", "Generate a new AI summary of GitHub commits", "Report"),
		molecules.NewMenuItem("⚙️  Setting", "Configure your API keys (Groq & Gemini)", "Setting"),
		molecules.NewMenuItem("❌ Exit", "Quit the application", "Exit"),
	}

	// Dashboard removed per user request

	for {
		m := pages.MenuModel{
			List:      list.New(items, list.NewDefaultDelegate(), 50, 14),
			LatestRel: latestRelease,
		}
		m.List.Title = "Main Menu"
		m.List.SetShowStatusBar(false)
		m.List.SetFilteringEnabled(false)
		m.List.SetShowHelp(false)

		p := tea.NewProgram(m)
		model, err := p.Run()
		if err != nil {
			fmt.Printf("Error running menu: %v", err)
			os.Exit(1)
		}

		finalModel := model.(pages.MenuModel)
		if finalModel.Choice == "Report" {
			report.Run(confPath)
		} else if finalModel.Choice == "Setting" {
			pages.RunSettings(confPath, pages.RunFilePickerWrapper(), pages.RunTemplateManagerWrapper())
		} else if finalModel.Choice == "Update" {
			updater.DoUpdate(latestRelease)
		} else {
			break
		}
	}
}
