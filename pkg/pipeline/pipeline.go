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

// ── Prompts untuk format simple per repo ─────────────────────────────────────

const MapSysPrompt = `Role: SE
Task: Konversi git commits ke daftar perubahan yang mendalam dan deskriptif
Language: Bahasa Indonesia
Rules:
- Format: "- deskripsi perubahan"
- Berikan detail teknis yang relevan dari pesan commit
- Jangan terlalu menyingkat; pastikan maksud perubahan tetap jelas dan akurat
- Fokus pada APA yang berubah dan MENGAPA (jika ada konteksnya)
- Gunakan bahasa Indonesia yang natural
- JANGAN gunakan prefix (feat/fix/chore/dll)
- JANGAN gunakan bold atau formatting markdown`

const ReduceSysPrompt = `Role: SE
Task: Gabungkan dan rapikan daftar perubahan dari berbagai chunks tanpa kehilangan detail
Language: Bahasa Indonesia
Rules:
- Format output:
- deskripsi perubahan
- deskripsi perubahan

- Hapus duplikat yang benar-benar identik
- Gabungkan perubahan yang mirip HANYA jika tidak menghilangkan detail spesifik
- Pastikan deskripsi yang dihasilkan tetap informatif dan deskriptif
- JANGAN gunakan prefix (feat/fix/chore/dll)
- JANGAN gunakan bold (**) atau formatting markdown
- JANGAN tambahkan penjelasan di luar format di atas`

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
