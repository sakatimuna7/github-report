package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"github-report-ai/internal/config"
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
Task: Analisis commit data dan buat laporan harian yang ringkas, akurat, dan mudah dipahami.
Language: Bahasa Indonesia
Context: {{CONTEXT}}

Format output yang WAJIB diikuti:

**[NAMA PROJECT/MODULE]**
- [prefix]: deskripsi perubahan dalam satu kalimat yang jelas dan ringkas
- [prefix]: deskripsi perubahan dalam satu kalimat yang jelas dan ringkas
...

Prefix yang digunakan:
- feat: untuk fitur baru
- fix: untuk perbaikan bug atau issue
- chore: untuk perubahan konfigurasi, dependency, atau maintenance
- refactor: untuk refactoring kode tanpa mengubah behavior
- perf: untuk optimasi performa
- docs: untuk perubahan dokumentasi
- style: untuk perubahan formatting kode
- test: untuk penambahan atau perbaikan test

Aturan penulisan:
1. Kelompokkan commits berdasarkan project/module (misal: "Tryout Kita", "Presensi", "API Gateway")
2. Setiap baris dimulai dengan "- [prefix]: " diikuti deskripsi
3. Deskripsi harus dalam 1 kalimat yang ringkas namun informatif
4. Fokus pada WHAT (apa yang berubah) dan WHY (kenapa penting), bukan HOW (detail teknis)
5. Gunakan bahasa Indonesia formal tapi natural
6. Hindari jargon teknis yang terlalu kompleks kecuali diperlukan
7. Jika ada breaking change, tambahkan indikator (BREAKING) setelah prefix
8. Urutkan dari yang paling penting/berdampak besar
9. Jangan mengulang informasi yang sama
10. JANGAN gunakan bold (**) kecuali untuk nama project/module
11. Gunakan kata kerja aktif: "menambahkan", "memperbaiki", "memperbarui", "menghapus"
12. Jika commit message tidak jelas, interpretasikan dengan konteks yang ada

Contoh format:
**Dashboard Admin**
- feat: menambahkan filter status pembayaran pada halaman daftar transaksi
- fix: memperbaiki bug pagination yang menampilkan data duplikat
- chore: memperbarui dependency React ke versi 18.3.0

**User Service**
- feat: mengimplementasikan reset password via email
- fix: memperbaiki validasi format nomor telepon Indonesia
- perf: mengoptimalkan query database dengan menambahkan index pada kolom email

PENTING: Output HARUS berupa daftar bullet points yang dikelompokkan per project, BUKAN dalam format bagian-bagian seperti SUMMARY, CHANGES, RISK, dll.`

// LoadTemplates ensures the templates directory exists and returns available templates.
func LoadTemplates() (map[string]string, error) {
	dir := filepath.Join(config.GetConfigBaseDir(), "templates")
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
	dir := filepath.Join(config.GetConfigBaseDir(), "templates")
	_ = os.MkdirAll(dir, 0755)
	return os.WriteFile(filepath.Join(dir, name+".txt"), []byte(content), 0644)
}

func DeleteTemplate(name string) error {
	path := filepath.Join(config.GetConfigBaseDir(), "templates", name+".txt")
	return os.Remove(path)
}
