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

// ── Ultra-compact prompts (schema-style) ─────────────────────────────────────

const MapSysPrompt = `Role: SE
Task: Summarize git commits
Rules:
- bullet per category (feat/fix/chore/refactor)
- for 'feat': DO NOT limit bullets, detail each feature as short sub-bullets (no long lines)
- include module if clear
- simple language
- max 5 bullets (except 'feat')`

const ReduceSysPrompt = `Role: SE
Task: Merge commit summaries
Rules:
- remove duplicates
- keep most important
- group by category
- for 'feat': keep detailed sub-bullets, do not limit
- max 7 bullets total (except 'feat')`

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
