package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"github-report-ai/internal/config"
)

var defaultTemplate = `Role: Senior Software Engineer
Task: Rangkum perubahan dari git commits menjadi laporan yang informatif dan deskriptif.
Language: Bahasa Indonesia
Context: {{CONTEXT}}

Format output WAJIB:
- deskripsi perubahan
- deskripsi perubahan

Aturan:
- Setiap baris adalah deskripsi perubahan dalam bahasa Indonesia yang natural
- Berikan penjelasan yang cukup detail agar pembaca paham apa yang sebenarnya berubah secara teknis
- Fokus pada APA yang berubah dan MENGAPA (gunakan Context atau Issue context jika tersedia)
- Gabungkan perubahan yang mirip HANYA jika tidak menghilangkan informasi penting
- JANGAN gunakan prefix (feat/fix/chore/dll)
- JANGAN gunakan bold (**) atau formatting markdown apapun
- JANGAN tambahkan penjelasan di luar format di atas

Contoh:
- Memperbarui logika validasi di dashboard untuk mendukung input dinamis
- Memperbaiki bug pada integrasi peta di mana marker tidak muncul setelah refresh, dan menambahkan fitur interaksi pada klik marker
- Melakukan refaktor pada modul master SHP untuk meningkatkan performa query data spasial`

// LoadTemplates ensures the templates directory exists and returns available templates.
func LoadTemplates() (map[string]string, error) {
	dir := filepath.Join(config.GetConfigBaseDir(), "templates")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
	}

	path := filepath.Join(dir, "Default.txt")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_ = os.WriteFile(path, []byte(defaultTemplate), 0644)
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
