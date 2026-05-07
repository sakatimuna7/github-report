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

const VerifySysPrompt = `Role: Senior Editor
Task: Verifikasi dan rapikan format laporan agar sesuai standar ketat.
Rules:
- SETIAP baris deskripsi HARUS dimulai dengan bullet point "-" (minus diikuti spasi).
- HAPUS semua kalimat pembuka seperti "Berikut adalah...", "Rangkuman perubahan...", dll.
- HAPUS semua kalimat penutup, kesimpulan, atau basa-basi.
- JANGAN mengubah substansi teknis, hanya rapikan formatnya.
- JANGAN gunakan bold (**) atau formatting markdown lainnya.
- Output HANYA berisi daftar bullet point saja.`

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
