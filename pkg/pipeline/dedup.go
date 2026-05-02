package pipeline

import (
	"regexp"
	"strings"
)

// commitLine represents one parsed commit entry.
type commitLine struct {
	Raw        string // original line text
	Normalized string // lowercased, stripped of hash/noise
}

// reHash matches a leading commit SHA (7–40 hex chars) commonly prefixed in messages.
var reHash = regexp.MustCompile(`^[0-9a-f]{7,40}\b`)

// reWhitespace collapses multiple whitespace into a single space.
var reWhitespace = regexp.MustCompile(`\s+`)

// normalizeCommitMsg produces a canonical version of a commit message for dedup.
// It strips leading hashes, lowercases, and collapses whitespace.
func normalizeCommitMsg(msg string) string {
	msg = strings.TrimSpace(msg)
	msg = strings.TrimPrefix(msg, "- ")
	msg = reHash.ReplaceAllString(msg, "")
	msg = strings.ToLower(msg)
	msg = reWhitespace.ReplaceAllString(msg, " ")
	msg = strings.TrimSpace(msg)
	return msg
}

// DeduplicateCommits removes duplicate commit lines from raw commit text.
// It preserves header lines (non "- " prefixed) and date sections "[YYYY-MM-DD]".
// Dedup is based on normalized message text (hash-agnostic).
//
// Returns: (deduplicated text, original count, deduplicated count, removed count)
func DeduplicateCommits(raw string) (string, int, int, int) {
	lines := strings.Split(raw, "\n")
	seen := make(map[string]struct{})
	var result []string
	origCount := 0
	dedupCount := 0

	for _, line := range lines {
		// Non-commit lines: headers, date sections, blank lines — always keep
		if !strings.HasPrefix(line, "- ") {
			result = append(result, line)
			continue
		}

		origCount++

		// Extract the message part after "- " and optional "(by Author)" suffix
		msg := line
		if idx := strings.LastIndex(msg, " (by "); idx > 0 {
			msg = msg[:idx]
		}
		norm := normalizeCommitMsg(msg)

		if _, exists := seen[norm]; exists {
			dedupCount++
			continue
		}
		seen[norm] = struct{}{}
		result = append(result, line)
	}

	return strings.Join(result, "\n"), origCount, origCount - dedupCount, dedupCount
}
