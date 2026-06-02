package report

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	ghLib "github.com/google/go-github/v62/github"
)

// filterOut returns slice with all elements except the one matching exclude.
func filterOut(s []string, exclude string) []string {
	out := s[:0:0]
	for _, v := range s {
		if v != exclude { out = append(out, v) }
	}
	return out
}

// isCleanBulletList returns true if every non-empty line starts with "- ".
// Used to skip Phase 4 Verify when the report is already properly formatted.
func isCleanBulletList(s string) bool {
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if l != "" && !strings.HasPrefix(l, "- ") { return false }
	}
	return true
}

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
	
	type repoInfo struct{ Owner, Repo, Branch string }
	var batchRepos []repoInfo
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
							ownerVal, repoVal := p[len(p)-2], p[len(p)-1]
							
							selectedBranches, _ := fetchAndSelectBranches(context.Background(), *tok, ownerVal, repoVal, nil)
							for _, b := range selectedBranches {
								batchRepos = append(batchRepos, repoInfo{Owner: ownerVal, Repo: repoVal, Branch: b})
							}
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
				
				currentBranch := ""
				if selected == fmt.Sprintf("%s/%s", localOwner, localRepo) {
					currentBranch = utils.Sh("git", "rev-parse", "--abbrev-ref", "HEAD")
				}

				selectedBranches, _ := fetchAndSelectBranches(context.Background(), *tok, *owner, *repo, []string{currentBranch})

				batchRepos = nil
				for _, b := range selectedBranches {
					batchRepos = append(batchRepos, repoInfo{Owner: *owner, Repo: *repo, Branch: b})
				}
				if len(selectedBranches) > 0 {
					*branch = selectedBranches[0]
				}
			}
			break
		}
	} else if *owner == "" || *repo == "" {
		if *owner == "" { *owner = localOwner }
		if *repo == "" { *repo = localRepo }
		batchRepos = []repoInfo{{*owner, *repo, ""}}
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
						huh.NewOption("gemini-2.5-flash (recommended)", "gemini-flash"),
						huh.NewOption("gemini-3.1-flash-lite (500 RPD, fastest)", "gemini-flash-lite3"),
						huh.NewOption("gemini-2.5-flash-lite (faster)", "gemini-flash-lite"),
						huh.NewOption("groq: llama-4-scout (best for code)", "groq-llama4"),
						huh.NewOption("groq: llama-3.3-70b (powerful)", "groq-llama"),
						huh.NewOption("groq: mixtral-8x7b", "groq-mixtral"),
						huh.NewOption("groq: gpt-oss-120b", "groq-gpt"),
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
		for _, br := range batchRepos {
			b := br.Branch; if b == "" { b = *branch }
			repoKey += br.Owner + "/" + br.Repo + "@" + b + "+"
		}
		cc := pipeline.NewFileCache(filepath.Join(cacheDir, pipeline.ContentHash(repoKey+s.Format("2006-01-02"))+"_chunks.json"))

		var allRaw []string
		var allCommitsList [][]*ghLib.RepositoryCommit
		var totalStats github.CommitStats
		spin.Suffix = color.HiBlackString(" Phase 1/4: 📡 Fetching GitHub Data...")
		spin.Start()
		
		ws, we := 9, 17
		if s := os.Getenv("WORK_START"); s != "" { fmt.Sscanf(s, "%d", &ws) }
		if s := os.Getenv("WORK_END"); s != "" { fmt.Sscanf(s, "%d", &we) }

		for _, br := range batchRepos {
			targetBranch := br.Branch
			if targetBranch == "" { targetBranch = *branch }
			raw, stats, commits, err := github.NewClient(*tok).GetReportData(c, br.Owner, br.Repo, targetBranch, *lim, s, u, ws, we)
			if err == nil {
				allRaw = append(allRaw, fmt.Sprintf("=== REPOSITORY: %s/%s ===\n%s", br.Owner, br.Repo, raw))
				allCommitsList = append(allCommitsList, commits)
				totalStats.Total += stats.Total
				totalStats.Features += stats.Features
				totalStats.Fixes += stats.Fixes
				totalStats.Overtime += stats.Overtime
			}
		}
		spin.Stop()

		// --- Commit Selection Feature ---
			if !*ciMode {
				var filteredRaw []string
				var filteredCommitsList [][]*ghLib.RepositoryCommit
				var filteredTotalStats github.CommitStats

				for idx, br := range batchRepos {
					if idx >= len(allCommitsList) {
						continue
					}
					commits := allCommitsList[idx]
					if len(commits) == 0 {
						filteredCommitsList = append(filteredCommitsList, commits)
						continue
					}

					var opts []huh.Option[string]
					var selectedSHAs []string
					for _, commit := range commits {
						sha := commit.GetSHA()
						shortSHA := sha
						if len(sha) > 7 {
							shortSHA = sha[:7]
						}
						msg := strings.Split(strings.TrimSpace(commit.GetCommit().GetMessage()), "\n")[0]
						author := commit.GetCommit().GetAuthor().GetName()
						date := commit.GetCommit().GetAuthor().GetDate().Format("02/01/06 15:04")

						// Check if overtime (outside work hours: 09:00-17:00)
						timeOnly := strings.Split(date, " ")[1]
						isOvertime := false
						if len(timeOnly) >= 5 {
							hour, err := strconv.Atoi(timeOnly[0:2])
							if err == nil && timeOnly[2] == ':' {
								if hour < ws || hour >= we {
									isOvertime = true
								}
							}
						}

						// Add overtime indicator
						overtimeTag := ""
						if isOvertime {
							overtimeTag = " (🔴 Overtime)"
						}

						label := fmt.Sprintf("[%s] %s (%s, by %s)%s", shortSHA, msg, date, author, overtimeTag)
						opts = append(opts, huh.NewOption(label, sha))
						selectedSHAs = append(selectedSHAs, sha)
					}

					// Add "Select All" option before other options
					opts = append([]huh.Option[string]{
						huh.NewOption("✅ Select All / Deselect All", "select-all"),
					}, opts...)

					var userSelectedSHAs []string
					err := huh.NewMultiSelect[string]().
						Title(fmt.Sprintf("Select Commits for %s/%s", br.Owner, br.Repo)).
						Description("Choose which commits to include in the report (space to select/deselect). Red text indicates overtime commits.").
						Options(opts...).
						Value(&userSelectedSHAs).
						Run()

					if err != nil {
						// User cancelled prompt, fallback to selecting all
						color.Yellow("⚠️ Commit selection cancelled, including all commits by default.")
						userSelectedSHAs = selectedSHAs
					}

					// Handle Select All / Deselect All
					if len(userSelectedSHAs) > 0 && userSelectedSHAs[0] == "select-all" {
						color.Cyan("🔄 Selecting all commits...")
						userSelectedSHAs = selectedSHAs
					}

					if len(userSelectedSHAs) == 0 {
						color.Red("❌ No commits selected! Proceeding with all commits instead.")
						userSelectedSHAs = selectedSHAs
					}

					// Filter commits
					var selectedCommits []*ghLib.RepositoryCommit
					shaMap := make(map[string]bool)
					for _, sha := range userSelectedSHAs {
						shaMap[sha] = true
					}
					for _, commit := range commits {
						if shaMap[commit.GetSHA()] {
							selectedCommits = append(selectedCommits, commit)
						}
					}

					// Rebuild stats and raw text
					repoRaw, repoStats := rebuildRepoData(br.Owner, br.Repo, br.Branch, selectedCommits, ws, we)

					filteredRaw = append(filteredRaw, fmt.Sprintf("=== REPOSITORY: %s/%s ===\n%s", br.Owner, br.Repo, repoRaw))
					filteredCommitsList = append(filteredCommitsList, selectedCommits)
					filteredTotalStats.Total += repoStats.Total
					filteredTotalStats.Features += repoStats.Features
					filteredTotalStats.Fixes += repoStats.Fixes
					filteredTotalStats.Overtime += repoStats.Overtime
				}

				allRaw = filteredRaw
				allCommitsList = filteredCommitsList
				totalStats = filteredTotalStats
			}


		// --- Feature: AI Security Auditor (runs concurrently with Phase 2) ---
		// Result is captured via channel so Phase 2 can start immediately
		var securityWarnings []string
		var securityWg sync.WaitGroup
		securityCh := make(chan []string, 1)
		if !*ciMode && len(allRaw) > 0 {
			securityWg.Add(1)
			go func() {
				defer securityWg.Done()
				key := *gm
				if strings.HasPrefix(*mod, "groq") {
					key = *gk
				}
				warnings, _ := AuditSecurity(c, *mod, key, strings.Join(allRaw, "\n"))
				securityCh <- warnings
			}()
		} else {
			securityCh <- nil
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
		// Collect security audit result (may already be done since it ran concurrently)
		securityWarnings = <-securityCh
		_ = securityWg // ensure goroutine completes before we read
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
						id := "gemini-2.5-flash"
						switch m {
						case "gemini-flash-lite":  id = "gemini-2.5-flash-lite"
						case "gemini-flash-lite3": id = "gemini-3.1-flash-lite"
						}
						res, use, err = ai.NewGeminiClient(*gm).GenerateReport(c, id, sp, d)
					} else {
						id := "llama-3.3-70b-versatile"
						if m == "groq-llama4" { id = "meta-llama/llama-4-scout-17b-16e-instruct" } else if m == "groq-mixtral" { id = "mixtral-8x7b-32768" } else if m == "groq-gpt" { id = "openai/gpt-oss-120b" }
						res, use, err = ai.NewGroqClient(*gk).GenerateReport(c, id, sp, d)
					}
					mu.Lock(); usage.PromptTokens += use.PromptTokens; usage.CompletionTokens += use.CompletionTokens; usage.TotalTokens += use.TotalTokens; mu.Unlock()
					return res, err
				}

				// fbDiff: tries user's chosen model first, falls back to gemini for reliability
				fbDiff := func(sp, d string) (string, error) {
					var errs []string
					var diffModels []string
					if strings.HasPrefix(*mod, "gemini") {
						// Gemini selected: chosen model first, then other gemini variants as fallback
						allGemini := []string{"gemini-flash", "gemini-flash-lite3", "gemini-flash-lite"}
						diffModels = append([]string{*mod}, filterOut(allGemini, *mod)...)
					} else {
						// Groq selected: try groq first, all gemini variants as fallback
						diffModels = []string{*mod, "gemini-flash", "gemini-flash-lite3", "gemini-flash-lite"}
					}
					for _, m := range diffModels {
						res, err := call(m, sp, d)
						if err == nil && res != "" { return res, nil }
						errs = append(errs, fmt.Sprintf("%s: %v", m, err))
					}
					return "", fmt.Errorf("%s", strings.Join(errs, " | "))
				}
				// fb: general fallback chain
				fb := func(pref, sp, d string) (string, error) {
					var errs []string
					if res, err := call(pref, sp, d); err == nil && res != "" { return res, nil } else { errs = append(errs, fmt.Sprintf("%s: %v", pref, err)) }
					for _, m := range []string{"gemini-flash", "gemini-flash-lite3", "gemini-flash-lite", "groq-llama4", "groq-llama"} {
						if m != pref {
							res, err := call(m, sp, d)
							if err == nil && res != "" { return res, nil }
							errs = append(errs, fmt.Sprintf("%s: %v", m, err))
						}
					}
					return "", fmt.Errorf("%s", strings.Join(errs, " | "))
				}

				finalReports := make([]string, len(allRaw))
				var wg sync.WaitGroup
				var diffErrs []string // collect error logs — printed after spinner stops, never exported
				spin.Suffix = color.HiBlackString(fmt.Sprintf(" Phase 2/4: \U0001f50d Starting deep analysis for %d repos...", len(allRaw)))
				spin.Restart()

				for i := range allRaw {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()
						br := batchRepos[idx]
						// mm = primary model for map/verify, rm = primary for reduce
						// Both use the user's selected model as primary
						mm, rm := *mod, *mod

						var commitSummaries []string

						// --- Phase 2/4: Commit Analysis (parallel, max 4 concurrent) ---
						if idx < len(allCommitsList) && len(allCommitsList[idx]) > 0 {
							commits := allCommitsList[idx]
							gh := github.NewClient(*tok)

							type commitResult struct {
								idx     int
								summary string
							}

							semaphore := make(chan struct{}, 4) // max 4 concurrent AI calls
							resultsCh := make(chan commitResult, len(commits))
							var commitWg sync.WaitGroup

							for j, cm := range commits {
								sha := cm.GetSHA()
								if sha == "" {
									continue
								}
								shortSHA := sha
								if len(sha) > 7 {
									shortSHA = sha[:7]
								}

								mu.Lock()
								spin.Suffix = color.HiBlackString(fmt.Sprintf(" Phase 2/4: 🔍 Analysing [%d/%d] %s", j+1, len(commits), shortSHA))
								mu.Unlock()

								// --- Check cache BEFORE spawning goroutine ---
								summaryKey := pipeline.ContentHash(fmt.Sprintf("diff-v7:%s/%s:%s", br.Owner, br.Repo, sha))
								if cached, ok := cc.Get(summaryKey); ok && len(cached) > 15 {
									resultsCh <- commitResult{j, cached}
									continue
								}

								commitWg.Add(1)
								go func(commitIdx int, cm interface{ GetSHA() string }, sha, shortSHA, summaryKey string) {
									defer commitWg.Done()
									semaphore <- struct{}{}
									defer func() { <-semaphore }()

									// Need the actual commit object — capture it from the outer range
									gcm := commits[commitIdx]

									rawMsg := strings.TrimSpace(gcm.GetCommit().GetMessage())
									firstLine := strings.Split(rawMsg, "\n")[0]
									rawMsg = strings.ReplaceAll(rawMsg, "\n", " ")

									// Fetch diff
									patch, patchErr := gh.GetCommitPatch(c, br.Owner, br.Repo, sha)

									var summary string
									if patchErr != nil || patch == "" {
										// No patch: use commit message as-is (no AI)
										summary = fmt.Sprintf("- %s", firstLine)
										resultsCh <- commitResult{commitIdx, summary}
										return
									}

									optimized := pipeline.CavemanDiff(patch)
									if optimized == "" {
										summary = fmt.Sprintf("- %s", firstLine)
										resultsCh <- commitResult{commitIdx, summary}
										return
									}

									// --- Normal path: send to AI ---
									if len(optimized) > 6000 {
										optimized = optimized[:6000]
									}
									input := fmt.Sprintf("COMMIT_MESSAGE: %s\nDIFF:\n%s", rawMsg, optimized)
									var aiErr error
									summary, aiErr = fbDiff(pipeline.DiffAnalyzeSysPrompt, input)
									if aiErr != nil || summary == "" {
										summary = fmt.Sprintf("- %s", firstLine)
									} else {
										cc.Set(summaryKey, summary)
									}
									resultsCh <- commitResult{commitIdx, summary}
								}(j, cm, sha, shortSHA, summaryKey)
							}

							commitWg.Wait()
							close(resultsCh)

							// Collect results in original order
							ordered := make([]string, len(commits))
							for r := range resultsCh {
								ordered[r.idx] = r.summary
							}
							for _, s := range ordered {
								if s != "" {
									commitSummaries = append(commitSummaries, s)
								}
							}
						}

						// --- Phase 3/4: Generate Report ---
						mu.Lock()
						spin.Suffix = color.HiBlackString(fmt.Sprintf(" Phase 3/4: 📝 Generating report for %s/%s...", br.Owner, br.Repo))
						mu.Unlock()

						var report string
						if len(commitSummaries) > 0 {
							// Always run AI reduce to produce richer, more detailed output
							report, _ = fb(rm, pipeline.ReduceSysPrompt, strings.Join(commitSummaries, "\n---\n"))
							if report == "" {
								report = strings.Join(commitSummaries, "\n")
							}
						} else {
							// Fallback: original text-based pipeline (when no commits available)
							dedup, _, _, _ := pipeline.DeduplicateCommits(allRaw[idx])
							chunks := pipeline.ChunkByChar(dedup, 2500)
							pool := pipeline.NewWorkerPool(5, cc)
							mRes := pool.Run(c, chunks, func(ctx context.Context, d string) (string, error) { return fb(mm, pipeline.MapSysPrompt, d) })
							sums, _ := pipeline.CollectSuccessful(mRes)
							report, _ = fb(rm, pipeline.ReduceSysPrompt, strings.Join(sums, "\n---\n"))
						}

						// Phase 4: Skipped — DiffAnalyzeSysPrompt + fast-path always produce clean "- " bullets
						finalReports[idx] = fmt.Sprintf("%s/%s (%s)\n%s", br.Owner, br.Repo, br.Branch, report)
					}(i)
				}
				wg.Wait(); _ = cc.Flush(); spin.Stop()

				// Print diff analysis errors/warnings after spinner — safe from overwrite, never exported
				if len(diffErrs) > 0 {
					fmt.Fprintf(os.Stderr, "\n%s\n", color.YellowString("── Diff Analysis Warnings ──────────────────"))
					for _, e := range diffErrs {
						fmt.Fprintln(os.Stderr, e)
					}
					fmt.Fprintln(os.Stderr)
				}

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
func fetchAndSelectBranches(ctx context.Context, tok, owner, repo string, defaultBranches []string) ([]string, error) {
	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	spin.Suffix = color.HiBlackString(" Fetching branches for %s/%s...", owner, repo)
	spin.Start()

	if tok == "" {
		tok = utils.Sh("gh", "auth", "token")
	}
	branches, bErr := github.NewClient(tok).ListBranches(ctx, owner, repo)
	spin.Stop()

	if bErr != nil || len(branches) == 0 {
		return nil, bErr
	}

	if len(branches) == 1 {
		return []string{branches[0]}, nil
	}

	var selectedBranches []string
	if len(defaultBranches) > 0 {
		// Filter default branches to only those that actually exist in the remote
		for _, db := range defaultBranches {
			for _, rb := range branches {
				if db == rb {
					selectedBranches = append(selectedBranches, db)
					break
				}
			}
		}
	}

	var bOpts []huh.Option[string]
	for _, brName := range branches {
		bOpts = append(bOpts, huh.NewOption(brName, brName))
	}
	err := huh.NewMultiSelect[string]().
		Title(fmt.Sprintf("Select Branches for %s/%s", owner, repo)).
		Description("Select one or more branches to include in the report").
		Options(bOpts...).
		Value(&selectedBranches).
		Run()

	return selectedBranches, err
}

func rebuildRepoData(owner, repo, branch string, commits []*ghLib.RepositoryCommit, workStart, workEnd int) (string, github.CommitStats) {
	stats := github.CommitStats{Total: len(commits)}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Repo: %s/%s | Branch: %s\n\n", owner, repo, branch))
	sb.WriteString(fmt.Sprintf("Total activity fetched: %d commits\n", len(commits)))

	lastDate := ""
	for _, commit := range commits {
		fullMsg := commit.GetCommit().GetMessage()
		lowerMsg := strings.ToLower(fullMsg)
		
		// Basic conventional commit detection
		if strings.HasPrefix(lowerMsg, "feat") {
			stats.Features++
		} else if strings.HasPrefix(lowerMsg, "fix") {
			stats.Fixes++
		} else if strings.HasPrefix(lowerMsg, "refactor") || strings.HasPrefix(lowerMsg, "perf") || strings.HasPrefix(lowerMsg, "style") {
			stats.Refactors++
		} else if strings.HasPrefix(lowerMsg, "docs") {
			stats.Docs++
		} else {
			stats.Others++
		}
		
		// Overtime check: Outside workStart to workEnd
		date := commit.GetCommit().GetAuthor().GetDate()
		hour := date.Hour()
		if hour >= workEnd || hour < workStart {
			stats.Overtime++
		}

		msg := strings.ReplaceAll(fullMsg, "\n", " ")
		author := commit.GetCommit().GetAuthor().GetName()
		fullDate := commit.GetCommit().GetAuthor().GetDate().Format("2006-01-02")
		
		if fullDate != lastDate {
			sb.WriteString(fmt.Sprintf("\n[%s]\n", fullDate))
			lastDate = fullDate
		}
		
		sb.WriteString(fmt.Sprintf("- %s (by %s)\n", msg, author))
	}

	return sb.String(), stats
}

