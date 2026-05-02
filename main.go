package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	_ "embed"
	"github-report-ai/pkg/ai"
	"github-report-ai/pkg/github"
	"github-report-ai/pkg/pipeline"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/manifoldco/promptui"
)

func sh(c string, a ...string) string {
	o, _ := exec.Command(c, a...).Output()
	return strings.TrimSpace(string(o))
}

func main() {
	_ = godotenv.Load()
	h, _ := os.UserHomeDir()
	confPath := h + "/.ghreport"
	if h != "" {
		encKey := getEncryptionKey(h)
		encData, err := os.ReadFile(confPath)
		if err == nil {
			dec := decrypt(string(encData), encKey)
			if dec != nil {
				m, _ := godotenv.Unmarshal(string(dec))
				for k, v := range m {
					if os.Getenv(k) == "" {
						os.Setenv(k, v)
					}
				}
			} else {
				// Fallback if not encrypted yet
				_ = godotenv.Load(confPath)
			}
		}
	}

	for {
		p := promptui.Select{
			Label: "GitHub Report AI",
			Items: []string{"🚀 Report", "⚙️  Setting", "❌ Exit"},
		}
		_, r, err := p.Run()
		if err != nil || r == "❌ Exit" {
			break
		}

		if r == "🚀 Report" {
			runReport(confPath)
		} else if r == "⚙️  Setting" {
			runSettings(confPath)
		}
	}
}

func runSettings(path string) {
	for {
		gk := os.Getenv("GROQ_API_KEY")
		gm := os.Getenv("GEMINI_API_KEY")

		gkS := color.RedString("[EMPTY]")
		if gk != "" {
			gkS = color.GreenString("[SET]")
		}
		gmS := color.RedString("[EMPTY]")
		if gm != "" {
			gmS = color.GreenString("[SET]")
		}

		p := promptui.Select{
			Label: "Settings",
			Items: []string{
				fmt.Sprintf("Groq API Key   %s", gkS),
				fmt.Sprintf("Gemini API Key %s", gmS),
				"⬅️  Back",
			},
		}

		idx, _, err := p.Run()
		if err != nil || idx == 2 {
			break
		}

		prompt := promptui.Prompt{Mask: '*'}
		if idx == 0 {
			prompt.Label = "Enter Groq API Key"
			res, _ := prompt.Run()
			if res != "" {
				os.Setenv("GROQ_API_KEY", res)
			}
		} else if idx == 1 {
			prompt.Label = "Enter Gemini API Key"
			res, _ := prompt.Run()
			if res != "" {
				os.Setenv("GEMINI_API_KEY", res)
			}
		}

		content := fmt.Sprintf("GROQ_API_KEY=%s\nGEMINI_API_KEY=%s\n", os.Getenv("GROQ_API_KEY"), os.Getenv("GEMINI_API_KEY"))
		
		h, _ := os.UserHomeDir()
		encKey := getEncryptionKey(h)
		encContent := encrypt([]byte(content), encKey)
		
		_ = os.WriteFile(path, []byte(encContent), 0600)
		fmt.Println(color.GreenString("✅ Saved!"))
	}
}

func getEncryptionKey(home string) []byte {
	keyPath := home + "/.ghreport.key"
	keyData, err := os.ReadFile(keyPath)
	if err == nil && len(keyData) == 32 {
		return keyData
	}
	key := make([]byte, 32)
	_, _ = io.ReadFull(rand.Reader, key)
	_ = os.WriteFile(keyPath, key, 0600)
	return key
}

func encrypt(data []byte, key []byte) string {
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	io.ReadFull(rand.Reader, nonce)
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(ciphertext)
}

func decrypt(cryptoText string, key []byte) []byte {
	data, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil
	}
	return plaintext
}



func runReport(confPath string) {
	owner := flag.String("owner", "", "")
	repo := flag.String("repo", "", "")
	branch := flag.String("branch", "", "")
	lim := flag.Int("limit", 0, "")
	tok := flag.String("token", os.Getenv("GITHUB_TOKEN"), "")
	gk := flag.String("groq-key", os.Getenv("GROQ_API_KEY"), "")
	gm := flag.String("gemini-key", os.Getenv("GEMINI_API_KEY"), "")
	mod := flag.String("ai", "gemini-flash", "")
	
	// Reset flags for re-runs if necessary, but flag.Parse can only be called once.
	// Since we are in a loop, we should probably handle flags differently or only once.
	if !flag.Parsed() {
		flag.Parse()
	}

	if *owner == "" || *repo == "" || (flag.NArg() > 0 && flag.Arg(0) == ".") {
		u := sh("git", "remote", "get-url", "origin")
		u = strings.TrimPrefix(strings.TrimPrefix(u, "https://github.com/"), "git@github.com:")
		u = strings.TrimSuffix(u, ".git")
		p := strings.Split(u, "/")
		if len(p) >= 2 {
			if *owner == "" {
				*owner = p[0]
			}
			if *repo == "" {
				*repo = p[1]
			}
		}
	}
	if *branch == "" {
		*branch = sh("git", "rev-parse", "--abbrev-ref", "HEAD")
	}
	if *tok == "" {
		*tok = sh("gh", "auth", "token")
	}

	sel := func(l string, i []string) string {
		p := promptui.Select{Label: l, Items: i}
		_, r, _ := p.Run()
		return r
	}

	*mod = sel("AI Model", []string{"gemini-flash", "gemini-flash-lite", "groq-llama", "groq-mixtral", "groq-gpt"})

	var d []string
	now := time.Now()
	for i := 0; i < 7; i++ {
		d = append(d, now.AddDate(0, 0, -i).Format("02/01/2006"))
	}
	d = append(d, "Custom Range")
	dr := sel("Date Period", d)

	var s, u time.Time
	if dr == "Custom Range" {
		p := promptui.Prompt{Label: "Since (YYYY-MM-DD)"}
		v, _ := p.Run()
		s, _ = time.Parse("2006-01-02", v)
		p.Label = "Until"
		v, _ = p.Run()
		if v != "" {
			u, _ = time.Parse("2006-01-02", v)
		}
	} else {
		sd, _ := time.Parse("02/01/2006", dr)
		s = time.Date(sd.Year(), sd.Month(), sd.Day(), 0, 0, 0, 0, sd.Location())
		u = time.Date(sd.Year(), sd.Month(), sd.Day(), 23, 59, 59, 0, sd.Location())
	}

	fr := sel("Focus", []string{"1. Semua", "2. Summary", "3. Changes", "4. Modules", "5. Authors", "6. Recs"})

	h, _ := os.UserHomeDir()
	cache := h + "/.ghreport_cache"
	_ = os.MkdirAll(cache, 0755)
	cc := pipeline.NewFileCache(cache + "/" + fmt.Sprintf("%s_%s_%s_%s_chunks.json", *owner, *repo, *branch, s.Format("2006-01-02")))

	color.Cyan("\n🚀 GITHUB REPORT AI\n====================")
	c := context.Background()
	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	spin.Suffix = " Fetching..."
	spin.Start()
	raw, err := github.NewClient(*tok).GetReportData(c, *owner, *repo, *branch, *lim, s, u)
	spin.Stop()
	if err != nil {
		fmt.Printf(color.RedString("Error: %v\n", err))
		return
	}

	ctxN, _ := (&promptui.Prompt{Label: "Context (optional)"}).Run()

	var usage ai.Usage
	var mu sync.Mutex
	call := func(m, sp, d string) (string, error) {
		var res string
		var use ai.Usage
		var err error
		if strings.HasPrefix(m, "gemini") {
			id := "gemini-2.0-flash"
			if m != "gemini-flash" {
				id = "gemini-2.0-flash-lite-preview-02-05"
			}
			res, use, err = ai.NewGeminiClient(*gm).GenerateReport(c, id, sp, d)
		} else {
			id := "llama-3.1-8b-instant"
			if m == "groq-mixtral" {
				id = "mixtral-8x7b-32768"
			} else if m == "groq-gpt" {
				id = "openai/gpt-oss-20b"
			}
			res, use, err = ai.NewGroqClient(*gk).GenerateReport(c, id, sp, d)
		}
		mu.Lock()
		usage.PromptTokens += use.PromptTokens
		usage.CompletionTokens += use.CompletionTokens
		usage.TotalTokens += use.TotalTokens
		mu.Unlock()
		return res, err
	}

	fb := func(pref, sp, d string) (string, error) {
		if res, err := call(pref, sp, d); err == nil {
			return res, nil
		}
		for _, m := range []string{"gemini-flash", "gemini-flash-lite", "groq-llama"} {
			if m != pref {
				if res, err := call(m, sp, d); err == nil {
					return res, nil
				}
			}
		}
		return "", fmt.Errorf("fail")
	}

	dedup, _, _, _ := pipeline.DeduplicateCommits(raw)
	chunks := pipeline.ChunkByChar(dedup, 2500)
	pool := pipeline.NewWorkerPool(5, cc)
	mm, rm := "gemini-flash-lite", "gemini-flash"
	if strings.HasPrefix(*mod, "groq") {
		mm, rm = "groq-llama", "groq-mixtral"
	}

	spin.Suffix = " MAP..."
	spin.Restart()
	mRes := pool.Run(c, chunks, func(ctx context.Context, d string) (string, error) { return fb(mm, pipeline.MapSysPrompt, d) })
	spin.Stop()
	sums, _ := pipeline.CollectSuccessful(mRes)
	_ = cc.Flush()

	spin.Suffix = " REDUCE..."
	spin.Restart()
	merged, _ := fb(rm, pipeline.ReduceSysPrompt, strings.Join(sums, "\n---\n"))

	fi := "Full report."
	if fr != "1. Semua" {
		fi = fr
	}
	sp := fmt.Sprintf("Role: SE\nTask: Format report\nLanguage: Bahasa Indonesia\nFocus: %s\nContext: %s\nRules: clean headings, bullet points, concise, NO bold tags (**text**)", fi, ctxN)

	report, _ := fb(mm, sp, merged)
	spin.Stop()

	fmt.Printf("\n%s\n%s\n%s\n%s\nUsage: P:%d C:%d T:%d\n", color.CyanString("✨ REPORT"), strings.Repeat("-", 40), report, strings.Repeat("-", 40), usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
}

