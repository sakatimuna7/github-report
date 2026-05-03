package pipeline

import (
	"os"
	"path/filepath"
	"strings"
)

var defaultTemplate = `Role: Senior Software Engineer
Task: Format the raw commit data into a clean, concise, and professional report.
Language: Bahasa Indonesia
Focus: {{FOCUS}}
Context: {{CONTEXT}}
Rules:
- Gunakan bullet points yang rapi.
- DILARANG menggunakan tag tebal/bold (**) pada teks biasa, HANYA boleh pada judul bagian.
- Buat se-ringkas mungkin namun informatif.
- Jangan mengulangi kalimat yang sama.`

// LoadTemplates ensures the templates directory exists and returns available templates.
func LoadTemplates() (map[string]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	
	dir := filepath.Join(home, ".ghreport_templates")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
		
		defaults := map[string]string{
			"Summary":   "Gambaran umum tentang progress yang dikerjakan.",
			"Changes":   "Fokus pada perubahan utama dan fitur baru yang diimplementasikan.",
			"Risk":      "Deteksi masalah, bug, atau risiko teknis dari commit data.",
			"Recommend": "Saran teknis atau improvement untuk kedepannya.",
			"Changelog": "Human-readable release notes untuk stakeholder.",
			"Default":   defaultTemplate,
		}

		for name, desc := range defaults {
			content := defaultTemplate
			if name != "Default" {
				content = strings.ReplaceAll(defaultTemplate, "Focus: {{FOCUS}}", "Focus: "+desc)
			}
			_ = os.WriteFile(filepath.Join(dir, name+".txt"), []byte(content), 0644)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	templates := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
			name := strings.TrimSuffix(entry.Name(), ".txt")
			content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err == nil {
				templates[name] = string(content)
			}
		}
	}
	
	if len(templates) == 0 {
		templates["Default"] = defaultTemplate
	}
	return templates, nil
}
