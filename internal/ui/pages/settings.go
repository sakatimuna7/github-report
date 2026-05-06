package pages

import (
	"fmt"
	"os"

	"github-report-ai/internal/config"
	"github-report-ai/internal/ui/molecules"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
)

// This will need some callbacks or shared state to work perfectly, 
// but let's start by moving the structural part.

func RunSettings(path string, runFilePicker func() string, runTemplateManager func()) {
	for {
		gk := os.Getenv("GROQ_API_KEY")
		gm := os.Getenv("GEMINI_API_KEY")

		gkS := color.RedString("Empty")
		if gk != "" {
			gkS = color.GreenString("Configured")
		}
		gmS := color.RedString("Empty")
		if gm != "" {
			gmS = color.GreenString("Configured")
		}

		ws := os.Getenv("WORK_START")
		we := os.Getenv("WORK_END")
		if ws == "" { ws = "9" }
		if we == "" { we = "17" }

		sID := os.Getenv("SHEETS_ID")
		dName := os.Getenv("DEVELOPER_NAME")
		divisi := os.Getenv("DIVISI")
		credPath := os.Getenv("GOOGLE_CREDENTIALS_PATH")

		sIdStr := color.RedString("Not Set")
		if sID != "" { sIdStr = color.GreenString("Set") }
		dNameStr := color.RedString("Not Set")
		if dName != "" { dNameStr = color.GreenString("Set") }
		divisiStr := color.RedString("Not Set")
		if divisi != "" { divisiStr = color.GreenString("Set") }
		
		credPathStr := color.RedString("Not Set")
		if credPath != "" { 
			credPathStr = color.GreenString("File Set") 
		}

		items := []list.Item{
			molecules.NewMenuItem("Groq API Key", "Status: " + gkS, "Groq"),
			molecules.NewMenuItem("Gemini API Key", "Status: " + gmS, "Gemini"),
			molecules.NewMenuItem("🕒 Work Start", "Currently: " + ws + ":00", "WorkStart"),
			molecules.NewMenuItem("🕔 Work End", "Currently: " + we + ":00", "WorkEnd"),
			molecules.NewMenuItem("📊 Sheets ID", "Status: " + sIdStr, "SheetsID"),
			molecules.NewMenuItem("🏢 Divisi", "Status: " + divisiStr, "Divisi"),
			molecules.NewMenuItem("👨💻 Dev Name", "Status: " + dNameStr, "DevName"),
			molecules.NewMenuItem("🔑 Google Creds", "Status: " + credPathStr, "CredPath"),
			molecules.NewMenuItem("🚀 Manage Templates", "Add/Delete Focus Templates", "Templates"),
			molecules.NewMenuItem("⬅️  Back", "Return to Main Menu", "Back"),
		}

		m := MenuModel{List: list.New(items, list.NewDefaultDelegate(), 50, 14)}
		m.List.Title = "Settings"
		m.List.SetShowStatusBar(false)
		m.List.SetFilteringEnabled(false)
		m.List.SetShowHelp(false)

		p := tea.NewProgram(m)
		model, err := p.Run()
		if err != nil {
			break
		}

		finalModel := model.(MenuModel)
		if finalModel.Quitting || finalModel.Choice == "Back" {
			break
		}

		var res string
		choice := finalModel.Choice
		if choice == "Groq" {
			huh.NewInput().Title("Enter Groq API Key").EchoMode(huh.EchoModePassword).Value(&res).Run()
			if res != "" { os.Setenv("GROQ_API_KEY", res) }
		} else if choice == "Gemini" {
			huh.NewInput().Title("Enter Gemini API Key").EchoMode(huh.EchoModePassword).Value(&res).Run()
			if res != "" { os.Setenv("GEMINI_API_KEY", res) }
		} else if choice == "WorkStart" {
			huh.NewInput().Title("Enter Work Start Hour (0-23)").Value(&res).Run()
			if res != "" { os.Setenv("WORK_START", res) }
		} else if choice == "WorkEnd" {
			huh.NewInput().Title("Enter Work End Hour (0-23)").Value(&res).Run()
			if res != "" { os.Setenv("WORK_END", res) }
		} else if choice == "SheetsID" {
			huh.NewInput().Title("Enter Google Spreadsheet ID").Value(&res).Run()
			if res != "" { os.Setenv("SHEETS_ID", res) }
		} else if choice == "Divisi" {
			huh.NewSelect[string]().
				Title("Pilih Divisi").
				Options(
					huh.NewOption("BACKEND", "BACKEND"),
					huh.NewOption("FRONTEND", "FRONTEND"),
					huh.NewOption("MOBILE", "MOBILE"),
				).
				Value(&res).
				Run()
			if res != "" { os.Setenv("DIVISI", res) }
		} else if choice == "DevName" {
			huh.NewInput().Title("Enter Your Developer Name").Value(&res).Run()
			if res != "" { os.Setenv("DEVELOPER_NAME", res) }
		} else if choice == "CredPath" {
			res = runFilePicker()
			if res != "" { os.Setenv("GOOGLE_CREDENTIALS_PATH", res) }
		} else if choice == "Templates" {
			runTemplateManager()
		}

		content := fmt.Sprintf("GROQ_API_KEY=%s\nGEMINI_API_KEY=%s\nWORK_START=%s\nWORK_END=%s\nSHEETS_ID=%s\nDIVISI=%s\nDEVELOPER_NAME=%s\nGOOGLE_CREDENTIALS_PATH=%s\n", 
			os.Getenv("GROQ_API_KEY"), os.Getenv("GEMINI_API_KEY"), 
			os.Getenv("WORK_START"), os.Getenv("WORK_END"),
			os.Getenv("SHEETS_ID"), os.Getenv("DIVISI"), os.Getenv("DEVELOPER_NAME"),
			os.Getenv("GOOGLE_CREDENTIALS_PATH"))
		
		if err := config.SaveEnv(path, content); err != nil {
			fmt.Println(color.RedString("❌ Failed to save: %v", err))
		} else {
			fmt.Println(color.GreenString("✅ Saved!"))
		}
	}
}
