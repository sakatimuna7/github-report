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

var dailyReportTemplate = `Role: Senior Software Engineer
Task: Analisis commit data dan buat laporan harian yang ringkas dan akurat.
Language: Bahasa Indonesia
Context: {{CONTEXT}}

Format output:

**SUMMARY**
2-3 kalimat ringkasan pencapaian hari ini dan status progress.

**CHANGES**
- [Feat] Fitur baru yang diselesaikan
- [Fix] Bug yang diperbaiki
- [Perf] Optimasi atau refactoring
- [Docs] Update dokumentasi (jika ada)

**RISK**
- Potensi bug atau breaking changes
- Technical debt yang teridentifikasi
- Area yang butuh testing lebih
Jika tidak ada: "Tidak ada risiko signifikan teridentifikasi."

**RECOMMEND**
- Prioritas testing atau review
- Task follow-up untuk besok
- Area yang perlu optimasi

**CHANGELOG**
Format release notes untuk stakeholder:
- Versi: [jika ada tag]
- Added: Fitur baru
- Fixed: Perbaikan bug
- Changed: Perubahan behavior
- Improved: Peningkatan performa

Rules:
- Bold HANYA untuk judul bagian
- Max 3-4 poin per bagian
- Fokus dampak bisnis, bukan detail teknis
- Hindari pengulangan informasi
- Jika bagian kosong, tulis 1 kalimat "Tidak ada [nama bagian]"`

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
		"Summary":     "Ringkasan eksekutif: pencapaian utama, progress, dan status proyek saat ini.",
		"Changes":     "Kategorikan perubahan: Feat (fitur), Fix (perbaikan), Perf (optimasi). Fokus dampak teknis.",
		"Risk":        "Identifikasi: breaking changes, potensi bug, technical debt, area berisiko.",
		"Recommend":   "Saran teknis: prioritas testing, area optimasi, task follow-up.",
		"Changelog":   "Release notes untuk stakeholder: Added, Fixed, Changed, Improved.",
		"DailyReport": dailyReportTemplate,
		"Default":     defaultTemplate,
	}

	for name, desc := range defaults {
		path := filepath.Join(dir, name+".txt")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			var content string
			if name == "DailyReport" {
				content = dailyReportTemplate
			} else if name != "Default" {
				content = strings.ReplaceAll(defaultTemplate, "Focus: {{FOCUS}}", "Focus: "+desc)
			} else {
				content = defaultTemplate
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
