package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github-report-ai/pkg/pipeline"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
)

var recommendedTemplates = map[string]bool{
	"Default":   true,
	"Summary":   true,
	"Changes":   true,
	"Risk":      true,
	"Recommend": true,
	"Changelog": true,
}

func runTemplateManager() {
	for {
		templates, _ := pipeline.LoadTemplates()
		var items []list.Item
		
		// Sort: Recommended first, then Custom
		for name := range templates {
			if recommendedTemplates[name] {
				items = append(items, menuItem{title: name, desc: color.GreenString("[Recommended]"), action: name})
			}
		}
		for name := range templates {
			if !recommendedTemplates[name] {
				items = append(items, menuItem{title: name, desc: color.CyanString("[Custom]"), action: name})
			}
		}

		items = append(items, menuItem{title: "➕ Add New Template", desc: "Create a custom focus", action: "ADD"})
		items = append(items, menuItem{title: "⬅️  Back", desc: "Return to Settings", action: "BACK"})

		m := menuModel{list: list.New(items, list.NewDefaultDelegate(), 50, 14)}
		m.list.Title = "Manage Focus Templates"
		m.list.SetShowStatusBar(false)
		m.list.SetFilteringEnabled(false)
		m.list.SetShowHelp(false)

		p := tea.NewProgram(m)
		model, err := p.Run()
		if err != nil {
			return
		}

		finalModel := model.(menuModel)
		choice := finalModel.choice
		if choice == "BACK" || finalModel.quitting {
			return
		}

		if choice == "ADD" {
			var name, focus string
			huh.NewForm(
				huh.NewGroup(
					huh.NewInput().Title("Template Name (e.g. Performance)").Value(&name),
					huh.NewInput().Title("Focus Description").Value(&focus),
				),
			).Run()

			if name != "" && focus != "" {
				home, _ := os.UserHomeDir()
				dir := filepath.Join(home, ".ghreport_templates")
				content := `Role: Senior Software Engineer
Task: Format the raw commit data into a clean, concise, and professional report.
Language: Bahasa Indonesia
Focus: ` + focus + `
Context: {{CONTEXT}}
Rules:
- Gunakan bullet points yang rapi.
- DILARANG menggunakan tag tebal/bold (**) pada teks biasa, HANYA boleh pada judul bagian.
- Buat se-ringkas mungkin namun informatif.`
				_ = os.WriteFile(filepath.Join(dir, name+".txt"), []byte(content), 0644)
				fmt.Println(color.GreenString("✅ Template %s created!", name))
			}
		} else {
			// Manage individual template
			var action string
			huh.NewSelect[string]().
				Title("Template: " + choice).
				Options(
					huh.NewOption("🗑️ Delete Template", "DELETE"),
					huh.NewOption("⬅️  Back", "BACK"),
				).
				Value(&action).
				Run()

			if action == "DELETE" {
				if recommendedTemplates[choice] {
					fmt.Println(color.RedString("❌ Cannot delete recommended templates!"))
					continue
				}
				var confirm bool
				huh.NewConfirm().Title("Delete template " + choice + "?").Value(&confirm).Run()
				if confirm {
					home, _ := os.UserHomeDir()
					dir := filepath.Join(home, ".ghreport_templates")
					_ = os.Remove(filepath.Join(dir, choice+".txt"))
					fmt.Println(color.GreenString("✅ Template %s deleted!", choice))
				}
			}
		}
	}
}
