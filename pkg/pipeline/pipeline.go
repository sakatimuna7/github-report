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
Task: Gabungkan daftar perubahan dari berbagai sumber menjadi satu daftar tanpa duplikat.
Language: Bahasa Indonesia
Rules:
- PERTAHANKAN SEMUA entri yang unik — jangan hilangkan atau ringkas perubahan apapun.
- HANYA hapus entri yang PERSIS IDENTIK secara kata per kata.
- DILARANG menggabungkan, meringkas, atau menggeneralisasi entri yang berbeda.
- DILARANG menambahkan interpretasi atau informasi baru yang tidak ada di input.
- Format: setiap baris diawali "- " (minus spasi).
- JANGAN gunakan bold (**) atau formatting markdown.
- JANGAN tambahkan kalimat pembuka, penutup, atau penjelasan apapun.
- Output HANYA berisi daftar bullet point dari input yang sudah ada.`

const VerifySysPrompt = `Role: Senior Editor
Task: Verifikasi dan rapikan format laporan agar sesuai standar ketat.
Rules:
- SETIAP baris deskripsi HARUS dimulai dengan bullet point "-" (minus diikuti spasi).
- HAPUS semua kalimat pembuka seperti "Berikut adalah...", "Rangkuman perubahan...", dll.
- HAPUS semua kalimat penutup, kesimpulan, atau basa-basi.
- JANGAN mengubah substansi teknis, hanya rapikan formatnya.
- JANGAN gunakan bold (**) atau formatting markdown lainnya.
- Output HANYA berisi daftar bullet point saja.`

const DiffAnalyzeSysPrompt = `Role: Senior Software Engineer
Task: Analisis kode perubahan (git diff) dan jelaskan apa yang terjadi secara teknis.
Language: Bahasa Indonesia
Rules:
- Analisis HANYA baris yang diawali (+) sebagai penambahan dan (-) sebagai penghapusan.
- Jelaskan perubahan logika atau fungsionalitas, BUKAN sekadar "menambah/menghapus baris".
- Sebutkan nama file dan fungsi yang berubah jika terlihat dari diff.
- Format: "- deskripsi perubahan teknis"
- JANGAN berhalusinasi. Jika diff tidak informatif, gunakan COMMIT_MESSAGE sebagai acuan.
- JANGAN buat informasi yang tidak ada di diff atau commit message.
- JANGAN gunakan bold (**) atau formatting markdown.
- Output HANYA daftar bullet point.`

// CavemanDiff strips boilerplate from git diffs, keeping only meaningful changes.
// It preserves file names for context and strips index/metadata lines.
func CavemanDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	var optimized []string
	for _, line := range lines {
		// Keep file names for context
		if strings.HasPrefix(line, "diff --git") {
			parts := strings.Split(line, " b/")
			if len(parts) >= 2 {
				optimized = append(optimized, "FILE: "+parts[len(parts)-1])
			}
			continue
		}
		// Keep hunk headers for location context
		if strings.HasPrefix(line, "@@") {
			optimized = append(optimized, line)
			continue
		}
		// Strip metadata lines
		if strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			continue
		}
		// Keep actual changes
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			optimized = append(optimized, line)
		}
	}
	return strings.Join(optimized, "\n")
}

// ToonEncode compresses key-value metadata into a compact delimited format.
func ToonEncode(data map[string]string) string {
	var parts []string
	for k, v := range data {
		v = strings.TrimSpace(v)
		if v != "" {
			parts = append(parts, k+":"+v)
		}
	}
	return strings.Join(parts, "|")
}

// SplitDiffByFile splits a large diff into per-file chunks.
// If the diff is smaller than maxChars, it returns a single-element slice.
func SplitDiffByFile(diff string, maxChars int) []string {
	if len(diff) <= maxChars {
		return []string{diff}
	}

	lines := strings.Split(diff, "\n")
	var chunks []string
	var current strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "FILE: ") && current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		current.WriteString(line + "\n")
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}

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
