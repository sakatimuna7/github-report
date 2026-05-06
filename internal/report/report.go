package report

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github-report-ai/internal/config"
	"github-report-ai/internal/ui/molecules"
	"github-report-ai/internal/ui/organisms"
	"github-report-ai/internal/utils"
	"github-report-ai/pkg/ai"
	"github-report-ai/pkg/github"
	"github-report-ai/pkg/pipeline"
	"github-report-ai/pkg/sheets"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
)

func Run(confPath string) {
	fs := flag.NewFlagSet("ghreport", flag.ContinueOnError)
	owner := fs.String("owner", "", "")
	repo := fs.String("repo", "", "")
	branch := fs.String("branch", "", "")
	lim := fs.Int("limit", 0, "")
	tok := fs.String("token", os.Getenv("GITHUB_TOKEN"), "")
	gk := fs.String("groq-key", os.Getenv("GROQ_API_KEY"), "")
	gm := fs.String("gemini-key", os.Getenv("GEMINI_API_KEY"), "")
	mod := fs.String("ai", "gemini-flash", "")
	ciMode := fs.Bool("ci", false, "")
	periodFlag := fs.String("period", "today", "Date period (e.g. 02/01/2006 or 'today')")
	focusFlag := fs.String("focus", "1. Semua", "Focus area")
	
	var batchRepos []struct{ Owner, Repo string }
	_ = fs.Parse(os.Args[1:])

	argsPassed := *owner != "" && *repo != ""

	// Try local git repo
	localOwner, localRepo := "", ""
	u := utils.Sh("git", "remote", "get-url", "origin")
	if u != "" {
		u = strings.TrimPrefix(strings.TrimPrefix(u, "https://github.com/"), "git@github.com:")
		u = strings.TrimSuffix(u, ".git")
		p := strings.Split(u, "/")
		if len(p) >= 2 {
			localOwner = p[0]
			localRepo = p[1]
		}
	}

	h := config.GetConfigBaseDir()
	historyPath := filepath.Join(h, "history.json")

	if !argsPassed && (fs.NArg() == 0 || fs.Arg(0) == ".") {
		if *ciMode {
			if localOwner == "" || localRepo == "" {
				color.Red("❌ Could not determine repository in CI mode. Use --owner and --repo flags.")
				return
			}
			*owner = localOwner
			*repo = localRepo
			goto skipMenuLoop
		}
	menuLoop:
		for {
			history, _ := pipeline.LoadRepoHistory(historyPath)
			
			var opts []huh.Option[string]
			if localOwner != "" && localRepo != "" {
				opts = append(opts, huh.NewOption(fmt.Sprintf("📍 Current Directory (%s/%s)", localOwner, localRepo), fmt.Sprintf("%s/%s", localOwner, localRepo)))
			}
			
			for _, hItem := range history {
				val := fmt.Sprintf("%s/%s", hItem.Owner, hItem.Repo)
				if val != fmt.Sprintf("%s/%s", localOwner, localRepo) {
					opts = append(opts, huh.NewOption(fmt.Sprintf("🕒 %s", val), val))
				}
			}

			if len(history) > 1 {
				opts = append(opts, huh.NewOption("📦 Batch Mode (Multi-Repo)", "batch"))
			}

			opts = append(opts, huh.NewOption("➕ Enter New Repository...", "new"))
			if len(history) > 0 {
				opts = append(opts, huh.NewOption("🗑️ Manage History (Delete Repo)", "manage"))
			}
			opts = append(opts, huh.NewOption("❌ Cancel", "exit"))

			var selected string
			err := huh.NewSelect[string]().
				Title("Select Repository to Analyze").
				Options(opts...).
				Value(&selected).
				Run()

			if err != nil || selected == "exit" {
				return
			}

			if selected == "batch" {
				var batchOpts []huh.Option[string]
				if localOwner != "" {
					val := fmt.Sprintf("%s/%s", localOwner, localRepo)
					batchOpts = append(batchOpts, huh.NewOption(val, val))
				}
				for _, hItem := range history {
					val := fmt.Sprintf("%s/%s", hItem.Owner, hItem.Repo)
					exists := false
					for _, o := range batchOpts {
						if o.Value == val {
							exists = true
							break
						}
					}
					if !exists {
						batchOpts = append(batchOpts, huh.NewOption(val, val))
					}
				}

				var batchSelected []string
				err = huh.NewMultiSelect[string]().
					Title("Select Repositories for Batch Report").
					Description("Choose 2 or more repos to combine into one report").
					Options(batchOpts...).
					Value(&batchSelected).
					Run()

				if err == nil && len(batchSelected) > 0 {
					batchRepos = nil
					for _, b := range batchSelected {
						p := strings.Split(b, "/")
						if len(p) >= 2 {
							batchRepos = append(batchRepos, struct{ Owner, Repo string }{p[0], p[1]})
						}
					}
					*owner = batchRepos[0].Owner
					*repo = batchRepos[0].Repo
					break
				}
				continue menuLoop
			}

			if selected == "manage" {
				var delOpts []huh.Option[string]
				for _, hItem := range history {
					val := fmt.Sprintf("%s/%s", hItem.Owner, hItem.Repo)
					delOpts = append(delOpts, huh.NewOption(fmt.Sprintf("🗑️ %s", val), val))
				}
				delOpts = append(delOpts, huh.NewOption("🔙 Back", "back"))

				var toDelete string
				err = huh.NewSelect[string]().
					Title("Select Repository to Delete from History").
					Options(delOpts...).
					Value(&toDelete).
					Run()

				if err == nil && toDelete != "back" {
					var newHistory []pipeline.RepoHistory
					for _, hItem := range history {
						val := fmt.Sprintf("%s/%s", hItem.Owner, hItem.Repo)
						if val != toDelete {
							newHistory = append(newHistory, hItem)
						}
					}
					_ = pipeline.SaveRepoHistory(historyPath, newHistory)
					color.Yellow("🗑️ Repository removed from history.")
				}
				continue menuLoop
			}

			if selected == "new" {
				err = huh.NewInput().
					Title("Enter GitHub URL or owner/repo").
					Value(&selected).
					Run()
				if err != nil || selected == "" {
					return
				}
			}

			uStr := selected
			uStr = strings.TrimPrefix(strings.TrimPrefix(uStr, "https://github.com/"), "git@github.com:")
			uStr = strings.TrimSuffix(uStr, ".git")
			parts := strings.Split(uStr, "/")
			if len(parts) >= 2 {
				*owner = parts[len(parts)-2]
				*repo = parts[len(parts)-1]
				batchRepos = []struct{ Owner, Repo string }{{*owner, *repo}}
			}
			break
		}
	} else if *owner == "" || *repo == "" {
		if *owner == "" { *owner = localOwner }
		if *repo == "" { *repo = localRepo }
		batchRepos = []struct{ Owner, Repo string }{{*owner, *repo}}
	}
skipMenuLoop:
	
	if *owner == "" || *repo == "" {
		color.Red("❌ Could not determine repository. Use --owner and --repo flags.")
		return
	}

	// Save to history
	history, _ := pipeline.LoadRepoHistory(historyPath)
	newHistory := []pipeline.RepoHistory{{Owner: *owner, Repo: *repo, LastUsed: time.Now()}}
	for _, hItem := range history {
		if hItem.Owner != *owner || hItem.Repo != *repo {
			newHistory = append(newHistory, hItem)
		}
	}
	if len(newHistory) > 10 {
		newHistory = newHistory[:10]
	}
	_ = pipeline.SaveRepoHistory(historyPath, newHistory)
	if *branch == "" {
		*branch = utils.Sh("git", "rev-parse", "--abbrev-ref", "HEAD")
	}
	if *tok == "" {
		*tok = utils.Sh("gh", "auth", "token")
	}

	var dr, fr, ctxN string

	for {
		if *ciMode {
			dr = *periodFlag
			fr = *focusFlag
			if dr == "today" {
				dr = time.Now().Format("02/01/2006")
			}
		} else {
			var dOpts []huh.Option[string]
			now := time.Now()
			for i := 0; i < 7; i++ {
				str := now.AddDate(0, 0, -i).Format("02/01/2006")
				dOpts = append(dOpts, huh.NewOption(str, str))
			}
			dOpts = append(dOpts, huh.NewOption("Custom Range", "Custom Range"))

			templates, _ := pipeline.LoadTemplates()
			var tOpts []huh.Option[string]
			for tName := range templates {
				tOpts = append(tOpts, huh.NewOption(tName, tName))
			}
			if len(tOpts) == 0 {
				tOpts = append(tOpts, huh.NewOption("Default", "Default"))
			}

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().Title("AI Model").Options(
						huh.NewOption("gemini-flash", "gemini-flash"),
						huh.NewOption("gemini-flash-lite", "gemini-flash-lite"),
						huh.NewOption("groq-llama", "groq-llama"),
						huh.NewOption("groq-mixtral", "groq-mixtral"),
						huh.NewOption("groq-gpt", "groq-gpt"),
					).Value(mod),
					huh.NewSelect[string]().Title("Date Period").Options(dOpts...).Value(&dr),
					huh.NewSelect[string]().Title("Template / Focus").Options(tOpts...).Value(&fr),
					huh.NewInput().Title("Context (optional)").Value(&ctxN),
				),
			)

			err := form.Run()
			if err != nil { return }
		}

		var s, u time.Time
		if dr == "Custom Range" {
			var since, until string
			err := huh.NewForm(huh.NewGroup(
				huh.NewInput().Title("Since (YYYY-MM-DD)").Value(&since),
				huh.NewInput().Title("Until (YYYY-MM-DD, Optional)").Value(&until),
			)).Run()
			if err != nil { return }
			s, _ = time.Parse("2006-01-02", since)
			if until != "" { u, _ = time.Parse("2006-01-02", until) } else { u = time.Now() }
		} else {
			sd, _ := time.Parse("02/01/2006", dr)
			s = time.Date(sd.Year(), sd.Month(), sd.Day(), 0, 0, 0, 0, sd.Location())
			u = time.Date(sd.Year(), sd.Month(), sd.Day(), 23, 59, 59, 0, sd.Location())
		}

		cacheDir := filepath.Join(h, "cache")
		_ = os.MkdirAll(cacheDir, 0755)

		c := context.Background()
		spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		_ = spin.Color("cyan", "bold")

		repoKey := ""
		for _, br := range batchRepos { repoKey += br.Owner + "/" + br.Repo + "+" }
		cc := pipeline.NewFileCache(filepath.Join(cacheDir, pipeline.ContentHash(repoKey+*branch+s.Format("2006-01-02"))+"_chunks.json"))

		var allRaw []string
		var totalStats github.CommitStats
		spin.Suffix = color.HiBlackString(" Fetching GitHub Data...")
		spin.Start()
		
		ws, we := 9, 17
		if s := os.Getenv("WORK_START"); s != "" { fmt.Sscanf(s, "%d", &ws) }
		if s := os.Getenv("WORK_END"); s != "" { fmt.Sscanf(s, "%d", &we) }

		for _, br := range batchRepos {
			raw, stats, err := github.NewClient(*tok).GetReportData(c, br.Owner, br.Repo, *branch, *lim, s, u, ws, we)
			if err == nil {
				allRaw = append(allRaw, fmt.Sprintf("=== REPOSITORY: %s/%s ===\n%s", br.Owner, br.Repo, raw))
				totalStats.Total += stats.Total
				totalStats.Features += stats.Features
				totalStats.Fixes += stats.Fixes
				totalStats.Overtime += stats.Overtime
			}
		}
		spin.Stop()

		// --- Feature: AI Security Auditor ---
		var securityWarnings []string
		if !*ciMode && len(allRaw) > 0 {
			spin.Suffix = color.HiBlackString(" Running AI Security Audit...")
			spin.Restart()
			key := *gm
			if strings.HasPrefix(*mod, "groq") {
				key = *gk
			}
			securityWarnings, _ = AuditSecurity(c, *mod, key, strings.Join(allRaw, "\n"))
			spin.Stop()
		}

		if len(allRaw) == 0 {
			color.Red("❌ No data fetched from any repository.")
			return
		}

		stats := totalStats
		columns := []table.Column{
			{Title: "Total Commits", Width: 15},
			{Title: "Features", Width: 10},
			{Title: "Fixes", Width: 10},
			{Title: "Overtime", Width: 10},
		}
		rows := []table.Row{{fmt.Sprintf("%d", stats.Total), fmt.Sprintf("%d", stats.Features), fmt.Sprintf("%d", stats.Fixes), fmt.Sprintf("%d", stats.Overtime)}}
		t := table.New(table.WithColumns(columns), table.WithRows(rows), table.WithFocused(false), table.WithHeight(3))


		purple := lipgloss.Color("99")
		st := table.DefaultStyles()
		st.Header = st.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(purple).BorderBottom(true).BorderLeft(true).BorderRight(true).BorderTop(true).Foreground(purple).Bold(true)
		st.Cell = st.Cell.BorderStyle(lipgloss.NormalBorder()).BorderForeground(purple).BorderLeft(true).BorderRight(true).BorderBottom(true)
		st.Selected = lipgloss.NewStyle()
		t.SetStyles(st)

		tableStyle := lipgloss.NewStyle().MarginBottom(1)
		summary := tableStyle.Render(t.View())

		reportsCacheDir := filepath.Join(h, "reports")
		_ = os.MkdirAll(reportsCacheDir, 0755)
		cacheKey := fmt.Sprintf("%s_%s_%s_%s", *owner, *repo, *branch, s.Format("2006-01-02"))
		cacheFile := filepath.Join(reportsCacheDir, pipeline.ContentHash(cacheKey)+".json")
		cachedResult, errCache := pipeline.LoadReportResult(cacheFile)
		hasCache := errCache == nil && time.Since(cachedResult.Timestamp) < 24*time.Hour

		selColumns := []table.Column{{Title: "AI Model", Width: 15}, {Title: "Period", Width: 15}, {Title: "Focus", Width: 15}}
		stbl := table.New(table.WithColumns(selColumns), table.WithRows([]table.Row{{*mod, dr, fr}}), table.WithFocused(false), table.WithHeight(3))
		stbl.SetStyles(st)
		selectionSummary := tableStyle.Render(stbl.View())

		var cacheNote string
		if hasCache {
			cacheNote = fmt.Sprintf("📅 Generated at: %s\n🤖 Model used: %s\n🎯 Focus: %s\n📊 Stats: %d commits, %d feats, %d fixes",
				cachedResult.Timestamp.Format("2006-01-02 15:04"), cachedResult.Model, cachedResult.Focus, cachedResult.TotalCommits, cachedResult.Features, cachedResult.Fixes)
		}

		var action string
		reportLoop:
		for {
			action = ""
			var fields []huh.Field
			fields = append(fields, huh.NewNote().Title("Review Selections").Description(selectionSummary))
			
			if len(securityWarnings) > 0 {
				warnMsg := color.RedString("⚠️  SECURITY ALERT: Potential secrets found in commits!\n\n") + strings.Join(securityWarnings, "\n")
				fields = append(fields, huh.NewNote().Title("🚨 Security Findings").Description(warnMsg))
			}

			contributionMap := molecules.RenderContributionMap(stats)
			petView := molecules.RenderPet(stats)
			statsView := lipgloss.JoinHorizontal(lipgloss.Top, contributionMap, "  ", petView)

			fields = append(fields, huh.NewNote().Title("Commit Statistics").Description(summary + "\n" + statsView))
			var proceed bool
			if hasCache {
				fields = append(fields, huh.NewNote().Title("✨ Cached Report Found").Description(cacheNote),
					huh.NewSelect[string]().Title("What would you like to do?").Options(huh.NewOption("Use Cached Report", "cache"), huh.NewOption("Regenerate (New AI Call)", "regen_ai"), huh.NewOption("Go Back / Cancel", "back")).Value(&action))
			} else if !*ciMode {
				fields = append(fields, huh.NewConfirm().Title("Proceed to generate AI report?").Affirmative("Yes, execute").Negative("No, go back").Value(&proceed))
			}

			if !*ciMode {
				err := huh.NewForm(huh.NewGroup(fields...)).Run()
				if err != nil { break reportLoop }
			}

			if *ciMode {
				if hasCache {
					action = "cache"
				} else {
					action = "regen_ai"
				}
			} else if !hasCache {
				if proceed {
					action = "regen_ai"
				} else {
					action = "back"
				}
			}
			if action == "back" || action == "" { break reportLoop }

			var reportContent string
			var usage ai.Usage
			
			if action == "cache" {
				reportContent = cachedResult.Content
				fmt.Println(color.GreenString("\n✅ Using cached report from %s", cachedResult.Timestamp.Format("2006-01-02 15:04")))
			} else {
				fmt.Println()
				color.Cyan("╭────────────────────────────────────────╮")
				color.Cyan("│ 🚀 GENERATING REPORT                   │")
				color.Cyan("╰────────────────────────────────────────╯")
				
				var mu sync.Mutex
				call := func(m, sp, d string) (string, error) {
					var res string
					var use ai.Usage
					var err error
					if strings.HasPrefix(m, "gemini") {
						id := "gemini-2.0-flash"; if m != "gemini-flash" { id = "gemini-2.0-flash-lite-preview-02-05" }
						res, use, err = ai.NewGeminiClient(*gm).GenerateReport(c, id, sp, d)
					} else {
						id := "llama-3.1-8b-instant"; if m == "groq-mixtral" { id = "mixtral-8x7b-32768" } else if m == "groq-gpt" { id = "openai/gpt-oss-20b" }
						res, use, err = ai.NewGroqClient(*gk).GenerateReport(c, id, sp, d)
					}
					mu.Lock(); usage.PromptTokens += use.PromptTokens; usage.CompletionTokens += use.CompletionTokens; usage.TotalTokens += use.TotalTokens; mu.Unlock()
					return res, err
				}

				fb := func(pref, sp, d string) (string, error) {
					if res, err := call(pref, sp, d); err == nil { return res, nil }
					for _, m := range []string{"gemini-flash", "gemini-flash-lite", "groq-llama"} {
						if m != pref { if res, err := call(m, sp, d); err == nil { return res, nil } }
					}
					return "", fmt.Errorf("fail")
				}

				finalReports := make([]string, len(allRaw))
				var wg sync.WaitGroup
				spin.Suffix = color.HiBlackString(fmt.Sprintf(" Analyzing %d repositories in parallel...", len(allRaw)))
				spin.Restart()

				for i := range allRaw {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()
						dedup, _, _, _ := pipeline.DeduplicateCommits(allRaw[idx])
						chunks := pipeline.ChunkByChar(dedup, 2500)
						pool := pipeline.NewWorkerPool(5, cc)
						mm, rm := "gemini-flash-lite", "gemini-flash"
						if strings.HasPrefix(*mod, "groq") { mm, rm = "groq-llama", "groq-mixtral" }
						mRes := pool.Run(c, chunks, func(ctx context.Context, d string) (string, error) { return fb(mm, pipeline.MapSysPrompt, d) })
						sums, _ := pipeline.CollectSuccessful(mRes)
						merged, _ := fb(rm, pipeline.ReduceSysPrompt, strings.Join(sums, "\n---\n"))
						templates, _ := pipeline.LoadTemplates()
						tmpl := templates[fr]; if tmpl == "" { tmpl = templates["Default"] }
						sp := strings.ReplaceAll(tmpl, "{{FOCUS}}", fr); sp = strings.ReplaceAll(sp, "{{CONTEXT}}", ctxN)
						report, _ := fb(mm, sp, merged)
						finalReports[idx] = fmt.Sprintf("## REPOSITORY: %s/%s\n\n%s", batchRepos[idx].Owner, batchRepos[idx].Repo, report)
					}(i)
				}
				wg.Wait(); _ = cc.Flush(); spin.Stop()

				repoNames := ""; for i, br := range batchRepos { if i > 0 { repoNames += ", " }; repoNames += br.Owner + "/" + br.Repo }
				headerPrefix := "# REPORT"; if len(batchRepos) > 1 { headerPrefix = "# BATCH REPORT" }
				reportContent = fmt.Sprintf("%s: %s\n\n%s\n\nUsage: %d Prompt | %d Completion | %d Total Tokens\n", headerPrefix, repoNames, strings.Join(finalReports, "\n\n---\n\n"), usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
				
				_ = pipeline.SaveReportResult(cacheFile, pipeline.ReportResult{
					Content: reportContent, Timestamp: time.Now(), Model: *mod, Period: dr, Focus: fr, TotalCommits: stats.Total, Features: stats.Features, Fixes: stats.Fixes, Overtime: stats.Overtime, Usage: fmt.Sprintf("%d Prompt | %d Completion | %d Total Tokens", usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens),
				})
			}

			var doExport bool
			if *ciMode && os.Getenv("SHEETS_ID") != "" && os.Getenv("DEVELOPER_NAME") != "" { doExport = true; fmt.Println(reportContent) } else if *ciMode { fmt.Println(reportContent); return }

			if !*ciMode {
				p := tea.NewProgram(organisms.ReportViewerModel{Content: reportContent}, tea.WithAltScreen(), tea.WithMouseCellMotion())
				model, err := p.Run()
				if err != nil { return }
				finalModel := model.(organisms.ReportViewerModel)
				if finalModel.Action == "regen" {
					hasCache = false
					continue reportLoop
				} else if finalModel.Action == "edit" {
					err := huh.NewForm(
						huh.NewGroup(
							huh.NewText().
								Title("Edit Report Content").
								Description("Modify the AI generated report (Press Ctrl+E for external editor)").
								Value(&reportContent).
								Lines(20),
						),
					).Run()
					if err != nil {
						continue reportLoop
					}
					continue reportLoop
				} else if finalModel.Action == "print" {
					fmt.Println("\n" + reportContent + "\n")
					continue reportLoop
				} else if finalModel.Action == "export_sheets" {
					doExport = true
				} else {
					break reportLoop
				}
			}
			if doExport {
				spin.Suffix = color.HiBlackString(" Exporting to Google Sheets..."); spin.Restart()
				sID := os.Getenv("SHEETS_ID"); dName := os.Getenv("DEVELOPER_NAME")
				if sID == "" || dName == "" { spin.Stop(); color.Red("❌ Missing SHEETS_ID or DEVELOPER_NAME"); continue reportLoop }
				credFile := os.Getenv("GOOGLE_CREDENTIALS_PATH"); if credFile == "" { credFile = filepath.Join(h, "google_credentials.json") }
				tokFile := filepath.Join(h, "google_token.json")
				srv, err := sheets.NewService(credFile, tokFile)
				if err != nil { spin.Stop(); color.Red("❌ Google Sheets Auth Error: %v", err); continue reportLoop }
				cleanContent := reportContent; if idx := strings.Index(cleanContent, "\n\nUsage:"); idx != -1 { cleanContent = cleanContent[:idx] }
				err = sheets.WriteReportToSheet(srv, sID, os.Getenv("DIVISI"), dName, s, cleanContent); spin.Stop()
				if err != nil { color.Red("❌ Failed to export: %v", err) } else { color.Green("✅ Successfully exported!") }
				continue reportLoop
			}
		}
	}
}
