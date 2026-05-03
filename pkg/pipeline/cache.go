package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"time"
)

// SummaryCache is an interface for caching LLM summaries by content hash.
// Implementations can be in-memory, file-backed, or Redis-backed.
type SummaryCache interface {
	// Get returns (summary, true) if a cache hit exists for the key.
	Get(key string) (string, bool)
	// Set stores a summary for the given key.
	Set(key string, value string)
	// Hits returns the number of cache hits so far.
	Hits() int
}

// ContentHash produces a deterministic SHA-256 hex hash for the given text.
func ContentHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

// ── In-memory cache ─────────────────────────────────────────────────────────

// MemoryCache is a thread-safe in-memory SummaryCache.
type MemoryCache struct {
	mu   sync.RWMutex
	data map[string]string
	hits int
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{data: make(map[string]string)}
}

func (c *MemoryCache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[key]
	if ok {
		c.hits++
	}
	return v, ok
}

func (c *MemoryCache) Set(key string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

func (c *MemoryCache) Hits() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits
}

// ── File-backed cache ───────────────────────────────────────────────────────

// FileCache persists summaries as a JSON map on disk and keeps them in memory.
type FileCache struct {
	path string
	mem  *MemoryCache
}

// NewFileCache loads an existing cache file (if any) and returns a FileCache.
func NewFileCache(path string) *FileCache {
	fc := &FileCache{
		path: path,
		mem:  NewMemoryCache(),
	}
	// Try to load existing cache
	if data, err := os.ReadFile(path); err == nil {
		var m map[string]string
		if json.Unmarshal(data, &m) == nil {
			fc.mem.mu.Lock()
			fc.mem.data = m
			fc.mem.mu.Unlock()
		}
	}
	return fc
}

func (c *FileCache) Get(key string) (string, bool) {
	return c.mem.Get(key)
}

func (c *FileCache) Set(key string, value string) {
	c.mem.Set(key, value)
}

func (c *FileCache) Hits() int {
	return c.mem.Hits()
}

// Flush writes the current cache state to disk. Call this after pipeline completion.
func (c *FileCache) Flush() error {
	c.mem.mu.RLock()
	data, err := json.Marshal(c.mem.data)
	c.mem.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0644)
}

// Size returns the number of entries in the cache.
func (c *FileCache) Size() int {
	c.mem.mu.RLock()
	defer c.mem.mu.RUnlock()
	return len(c.mem.data)
}

// ReportResult represents a fully generated report with its metadata.
type ReportResult struct {
	Content      string    `json:"content"`
	Timestamp    time.Time `json:"timestamp"`
	Model        string    `json:"model"`
	Period       string    `json:"period"`
	Focus        string    `json:"focus"`
	TotalCommits int       `json:"total_commits"`
	Features     int       `json:"features"`
	Fixes        int       `json:"fixes"`
	Overtime     int       `json:"overtime"`
	Usage        string    `json:"usage"` // Token usage info
}

// SaveReportResult saves a report to a file.
func SaveReportResult(path string, result ReportResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadReportResult loads a report from a file.
func LoadReportResult(path string) (ReportResult, error) {
	var result ReportResult
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}

// RepoHistory tracks a previously analyzed repository
type RepoHistory struct {
	Owner    string    `json:"owner"`
	Repo     string    `json:"repo"`
	LastUsed time.Time `json:"last_used"`
}

// LoadRepoHistory loads the repository history from a JSON file
func LoadRepoHistory(path string) ([]RepoHistory, error) {
	var history []RepoHistory
	data, err := os.ReadFile(path)
	if err != nil {
		return history, err
	}
	err = json.Unmarshal(data, &history)
	return history, err
}

// SaveRepoHistory saves the repository history to a JSON file
func SaveRepoHistory(path string, history []RepoHistory) error {
	data, err := json.Marshal(history)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
