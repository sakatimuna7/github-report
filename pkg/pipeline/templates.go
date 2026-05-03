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
	}

	defaults := map[string]string{
		"Summary":   "Berikan ringkasan eksekutif tentang kemajuan proyek, pencapaian utama, dan status saat ini dalam bahasa yang profesional.",
		"Changes":   "Identifikasi perubahan fitur (Feat), perbaikan (Fix), dan peningkatan (Perf). Jelaskan dampak teknis dari perubahan tersebut.",
		"Risk":      "Analisis potensi resiko, bug, atau hutang teknis yang mungkin muncul dari pola perubahan kode yang ada.",
		"Recommend": "Berikan rekomendasi teknis untuk langkah selanjutnya atau area yang memerlukan optimasi berdasarkan aktivitas commit.",
		"Changelog": "Buat catatan rilis (Release Notes) yang rapi, informatif, dan mudah dipahami oleh stakeholder non-teknis.",
		"Default":   defaultTemplate,
	}

	for name, desc := range defaults {
		path := filepath.Join(dir, name+".txt")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			content := defaultTemplate
			if name != "Default" {
				content = strings.ReplaceAll(defaultTemplate, "Focus: {{FOCUS}}", "Focus: "+desc)
			}
			_ = os.WriteFile(path, []byte(content), 0644)
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

func SaveTemplate(name, content string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".ghreport_templates")
	_ = os.MkdirAll(dir, 0755)
	return os.WriteFile(filepath.Join(dir, name+".txt"), []byte(content), 0644)
}

func DeleteTemplate(name string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".ghreport_templates", name+".txt")
	return os.Remove(path)
}
