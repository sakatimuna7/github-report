package pipeline

import (
	"strings"
)

// ChunkByChar splits raw text into chunks of ~maxChars, cutting at newline boundaries.
func ChunkByChar(raw string, maxChars int) []string {
	var result []string
	for len(raw) > maxChars {
		cut := maxChars
		if idx := strings.LastIndex(raw[:cut], "\n"); idx > 0 {
			cut = idx + 1
		}
		result = append(result, raw[:cut])
		raw = raw[cut:]
	}
	if raw != "" {
		result = append(result, raw)
	}
	return result
}

// ── Prompts untuk format changelog style ─────────────────────────────────────

const MapSysPrompt = `Role: SE
Task: Konversi git commits ke format changelog
Language: Bahasa Indonesia
Rules:
- Format: "- [prefix]: deskripsi singkat dan jelas"
- Prefix: feat/fix/chore/refactor/perf/docs/style/test
- Satu kalimat per bullet, maksimal 80 karakter
- Fokus pada WHAT (apa yang berubah) bukan HOW (detail teknis)
- Gunakan kata kerja aktif: menambahkan, memperbaiki, memperbarui, menghapus
- Jika ada module/fitur yang jelas, sebutkan di awal deskripsi
- Hindari jargon teknis yang tidak perlu
- JANGAN batasi jumlah bullets
- JANGAN gunakan bold atau formatting markdown`

const ReduceSysPrompt = `Role: SE
Task: Gabungkan dan rapikan changelog dari berbagai chunks
Language: Bahasa Indonesia
Rules:
- Kelompokkan per repository/project jika ada (gunakan **Nama Project**)
- Di dalam setiap project, urutkan: feat → fix → chore → refactor → perf → docs
- Format: "- [prefix]: deskripsi"
- Hapus duplikat yang persis sama
- Gabungkan commits yang sangat mirip menjadi satu
- Prioritaskan perubahan yang paling berdampak
- Pertahankan semua informasi penting, JANGAN potong fitur
- Maksimal 30 bullets total (kecuali ada banyak fitur penting)
- Output format:

**[Nama Project 1]**
- feat: deskripsi
- feat: deskripsi
- fix: deskripsi
- chore: deskripsi

**[Nama Project 2]**
- feat: deskripsi
- fix: deskripsi

- Jika hanya 1 repository, langsung list tanpa header project
- JANGAN tambahkan penjelasan atau komentar di luar format bullets`

// Stats holds pipeline execution statistics.
type Stats struct {
	OrigCommits   int
	DedupCommits  int
	RemovedDups   int
	TotalChunks   int
	CacheHits     int
	MapSuccessful int
	MapErrors     int
}
