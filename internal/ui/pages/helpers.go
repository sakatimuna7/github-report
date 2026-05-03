package pages

import (
	"time"

	"github-report-ai/pkg/pipeline"
	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
)

func runFilePicker() string {
	var path string
	err := huh.NewInput().
		Title("Enter Absolute Path to Google Credentials JSON").
		Placeholder("/Users/name/Downloads/credentials.json").
		Value(&path).
		Run()
	if err != nil {
		return ""
	}
	return path
}

func runTemplateManager() {
	for {
		templates, _ := pipeline.LoadTemplates()
		var opts []huh.Option[string]
		
		// Sort keys for consistent order
		var keys []string
		for k := range templates {
			keys = append(keys, k)
		}
		
		for _, k := range keys {
			label := k
			if k == "Default" || k == "summary" || k == "changes" || k == "risk" || k == "recommend" || k == "changelog" {
				label = "⭐ " + k + " (Recommended)"
			}
			opts = append(opts, huh.NewOption(label, k))
		}
		opts = append(opts, huh.NewOption("➕ Add New Template", "add"))
		opts = append(opts, huh.NewOption("⬅️  Back", "back"))

		var selected string
		err := huh.NewSelect[string]().
			Title("Template Manager").
			Options(opts...).
			Value(&selected).
			Run()

		if err != nil || selected == "back" {
			break
		}

		if selected == "add" {
			var name, content string
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().Title("Template Name").Value(&name),
					huh.NewText().Title("Template Content").
						Description("Use {{FOCUS}} and {{CONTEXT}} placeholders").
						Value(&content),
				),
			).Run()
			if err == nil && name != "" && content != "" {
				_ = pipeline.SaveTemplate(name, content)
				color.Green("✅ Template added!")
			}
		} else {
			// Manage existing
			var action string
			huh.NewSelect[string]().
				Title("Manage: " + selected).
				Options(
					huh.NewOption("📝 Edit", "edit"),
					huh.NewOption("🗑️ Delete", "delete"),
					huh.NewOption("🔙 Back", "back"),
				).
				Value(&action).
				Run()

			if action == "edit" {
				content := templates[selected]
				err = huh.NewText().Title("Edit Template").Value(&content).Run()
				if err == nil {
					_ = pipeline.SaveTemplate(selected, content)
					color.Green("✅ Template updated!")
				}
			} else if action == "delete" {
				if selected == "Default" {
					color.Red("❌ Cannot delete Default template.")
					time.Sleep(1 * time.Second)
					continue
				}
				confirm := false
				huh.NewConfirm().Title("Delete template '" + selected + "'?").Value(&confirm).Run()
				if confirm {
					_ = pipeline.DeleteTemplate(selected)
					color.Green("✅ Template deleted!")
				}
			}
		}
	}
}

func RunFilePickerWrapper() func() string {
	return runFilePicker
}

func RunTemplateManagerWrapper() func() {
	return runTemplateManager
}

func GetRunSettings(path string) func() {
	return func() {
		RunSettings(path, runFilePicker, runTemplateManager)
	}
}
