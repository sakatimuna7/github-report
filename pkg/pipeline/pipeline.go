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

const MapSysPrompt = `Role:SE|Lang:ID
Task:Konversi commit messages ke daftar perubahan teknis deskriptif.
- Format:"- deskripsi". Detail APA berubah & MENGAPA jika ada. Bahasa natural.
- JANGAN prefix feat/fix/chore. Output HANYA bullet list tanpa markdown/bold.`

const ReduceSysPrompt = `Role:SE|Lang:ID
Task:Gabungkan daftar perubahan menjadi laporan teknis yang informatif. Kelompokkan item sejenis (misal: semua perubahan UI, semua bugfix) dalam 1 bullet yang deskriptif.
- Hapus entri yang IDENTIK, tapi pertahankan detail teknis penting meskipun mirip.
- Setiap bullet harus informatif: APA yang berubah, file/modul terkait, dan DAMPAKNYA jika ada.
- Format:"- deskripsi detail". Output HANYA bullet list tanpa pembuka/penutup/markdown.`

const VerifySysPrompt = `Role:Editor
Task:Pastikan setiap baris dimulai "- ". Hapus kalimat pembuka/penutup/basa-basi.
- Jangan ubah substansi teknis. Output HANYA bullet list tanpa markdown.`

const DiffAnalyzeSysPrompt = `Role:SE|Lang:ID
Task:Baca git diff & commit message, buat ringkasan teknis perubahan dalam 1-3 bullet.
- Fokus pada APA yang berubah (fitur/bugfix/refactor/config), file/modul terkait, dan MENGAPA jika terlihat dari diff.
- Jelaskan perubahan logis/fungsional, bukan sekadar nama file. Contoh: "Menambahkan validasi input pada form login (auth/login.go)"
- Format:"- deskripsi teknis". Jika diff tidak informatif, gunakan COMMIT_MESSAGE.
- JANGAN halusinasi. Output HANYA bullet list, max 3 baris, tanpa markdown.`

// noiseFileSuffixes lists file patterns that add no value to diff analysis.
var noiseFileSuffixes = []string{
	"go.sum", "go.lock", "package-lock.json", "yarn.lock",
	"pnpm-lock.yaml", "Gemfile.lock", "Cargo.lock",
	".min.js", ".min.css", ".js.map", ".css.map",
}

func isNoiseFile(name string) bool {
	for _, suffix := range noiseFileSuffixes {
		if strings.HasSuffix(name, suffix) { return true }
	}
	return false
}

// CavemanDiff strips boilerplate from git diffs, keeping only meaningful changes.
// Filters out noise files (lock files, minified assets) and hunk headers.
func CavemanDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	var optimized []string
	skipFile := false
	for _, line := range lines {
		// File boundary
		if strings.HasPrefix(line, "diff --git") {
			parts := strings.Split(line, " b/")
			if len(parts) >= 2 {
				fname := parts[len(parts)-1]
				skipFile = isNoiseFile(fname)
				if !skipFile {
					optimized = append(optimized, "FILE: "+fname)
				}
			}
			continue
		}
		if skipFile { continue }
		// Strip metadata and hunk headers — AI only needs file name + +/- lines
		if strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "@@") {
			continue
		}
		// Keep actual changes only
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

// ExtractFilesFromDiff returns file names touched in a CavemanDiff output.
// Used for building a lightweight no-AI summary for small/trivial diffs.
func ExtractFilesFromDiff(diff string) []string {
	var files []string
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "FILE: ") {
			files = append(files, strings.TrimPrefix(line, "FILE: "))
		}
	}
	return files
}

