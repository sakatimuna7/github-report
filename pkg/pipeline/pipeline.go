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
Task:Gabungkan daftar perubahan; hapus HANYA entri identik persis kata per kata.
- Pertahankan semua entri unik. JANGAN ringkas/gabung/interpretasi entri berbeda.
- Format:"- item". Output HANYA bullet list tanpa pembuka/penutup/markdown.`

const VerifySysPrompt = `Role:Editor
Task:Pastikan setiap baris dimulai "- ". Hapus kalimat pembuka/penutup/basa-basi.
- Jangan ubah substansi teknis. Output HANYA bullet list tanpa markdown.`

const DiffAnalyzeSysPrompt = `Role:SE|Lang:ID
Task:Analisis git diff, jelaskan perubahan teknis & logika secara deskriptif.
- Fokus baris +/- saja. Sebut file & fungsi jika terlihat dari diff.
- Format:"- deskripsi". Jika diff tidak informatif, gunakan COMMIT_MESSAGE.
- JANGAN halusinasi. Output HANYA bullet list tanpa markdown.`

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
