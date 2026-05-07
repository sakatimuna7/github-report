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
Task: Konversi git commits ke daftar perubahan singkat
Language: Bahasa Indonesia
Rules:
- Format: "- deskripsi perubahan"
- Satu kalimat per bullet, ringkas dan jelas
- Fokus pada APA yang berubah, bukan bagaimana caranya
- Gunakan bahasa Indonesia yang natural
- JANGAN gunakan prefix (feat/fix/chore/dll)
- JANGAN gunakan bold atau formatting markdown`

const ReduceSysPrompt = `Role: SE
Task: Gabungkan dan rapikan daftar perubahan dari berbagai chunks
Language: Bahasa Indonesia
Rules:
- Format output:
- deskripsi perubahan
- deskripsi perubahan

- Hapus duplikat yang persis sama
- Gabungkan perubahan yang sangat mirip menjadi satu bullet
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
