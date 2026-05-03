package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github-report-ai/internal/config"
	"github-report-ai/internal/report"
	"github-report-ai/internal/ui/molecules"
	"github-report-ai/internal/ui/pages"
	"github-report-ai/internal/updater"
	"github-report-ai/internal/utils"
	"github-report-ai/pkg/github"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/joho/godotenv"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
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

	if isCI {
		report.Run(confPath)
		return
	}

	items := []list.Item{
		molecules.NewMenuItem("🚀 Report", "Generate a new AI summary of GitHub commits", "Report"),
		molecules.NewMenuItem("⚙️  Setting", "Configure your API keys (Groq & Gemini)", "Setting"),
		molecules.NewMenuItem("❌ Exit", "Quit the application", "Exit"),
	}

	var dashData github.DashboardData
	var dashLoaded bool
	var dashErr error
	var username string

	tok := os.Getenv("GITHUB_TOKEN")
	if tok == "" {
		tok = os.Getenv("GH_TOKEN")
	}
	if tok == "" {
		tok = utils.Sh("gh", "auth", "token")
	}

	if tok != "" {
		c := context.Background()
		gh := github.NewClient(tok)
		username, _ = gh.GetUserLogin(c)
		
		if username != "" {
			dashCachePath := h + "/.ghreport_dashboard.json"
			type DashCache struct {
				Data      github.DashboardData
				Timestamp time.Time
			}
			var cache DashCache
			if data, err := os.ReadFile(dashCachePath); err == nil {
				if json.Unmarshal(data, &cache) == nil {
					if time.Since(cache.Timestamp) < 24*time.Hour {
						dashData = cache.Data
						dashLoaded = true
					}
				}
			}

			if !dashLoaded {
				dashData, dashErr = gh.GetDashboardData(c, username)
				if dashErr == nil {
					dashLoaded = true
					cache = DashCache{Data: dashData, Timestamp: time.Now()}
					if b, err := json.Marshal(cache); err == nil {
						_ = os.WriteFile(dashCachePath, b, 0644)
					}
				}
			}
		}
	}

	for {
		m := pages.MenuModel{
			List:       list.New(items, list.NewDefaultDelegate(), 50, 14),
			DashData:   dashData,
			DashLoaded: dashLoaded,
			LatestRel:  latestRelease,
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
