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
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
	latestRelease *selfupdate.Release
)

func printVersion() {
	fmt.Printf("ghreport version %s\n", Version)
	fmt.Printf("commit: %s\n", Commit)
	fmt.Printf("built at: %s\n", BuildTime)
}

func sh(c string, a ...string) string {
	o, _ := exec.Command(c, a...).Output()
	return strings.TrimSpace(string(o))
}

func getBanner() string {
	banner := `   ____ _ _   _   _       _     ____                       _   
  / ___(_) |_| | | |_   _| |__ |  _ \ ___ _ __   ___  _ __| |_ 
 | |  _| | __| |_| | | | | '_ \| |_) / _ \ '_ \ / _ \| '__| __|
 | |_| | | |_|  _  | |_| | |_) |  _ <  __/ |_) | (_) | |  | |_ 
  \____|_|\__|_| |_|\__,_|_.__/|_| \_\___| .__/ \___/|_|   \__|
                                         |_|                   `

	cyan := lipgloss.NewStyle().Foreground(lipgloss.Color("36"))  // Cyan
	gray := lipgloss.NewStyle().Foreground(lipgloss.Color("241")) // Dark Gray

	return "\n" + cyan.Render(banner) + "\n" +
		gray.Render("   AI-Powered GitHub Commit Summarizer") + "\n" +
		gray.Render("   ===================================") + "\n"
}

type menuItem struct {
	title, desc string
	action      string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type menuModel struct {
	list     list.Model
	choice   string
	quitting bool
}

func (m menuModel) Init() tea.Cmd {
	return nil
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			i, ok := m.list.SelectedItem().(menuItem)
			if ok {
				m.choice = i.action
			}
			return m, tea.Quit
		case "u":
			if latestRelease != nil {
				m.choice = "Update"
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		bannerHeight := 10
		if latestRelease != nil {
			bannerHeight += 4
		}
		m.list.SetSize(msg.Width-h, msg.Height-v-bannerHeight)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m menuModel) View() string {
	if m.quitting || m.choice != "" {
		return ""
	}
	
	view := getBanner()
	if latestRelease != nil {
		noticeStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42")).
			Padding(0, 1).
			MarginLeft(2)
		
		notice := fmt.Sprintf("🎉 Update Available: %s\nPress 'u' to update now", latestRelease.Version())
		view += noticeStyle.Render(notice) + "\n"
	}
	
	return view + "\n" + docStyle.Render(m.list.View())
}

type reportViewerModel struct {
	viewport viewport.Model
	ready    bool
	content  string
	action   string
	toast    string
}

func (m reportViewerModel) Init() tea.Cmd {
	return nil
}

func (m reportViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch k := msg.String(); k {
		case "ctrl+c", "q", "esc":
			m.action = "quit"
			return m, tea.Quit
		case "r":
			m.action = "regen"
			return m, tea.Quit
		case "p":
			m.action = "print"
			return m, tea.Quit
		case "c":
			c := exec.Command("pbcopy")
			c.Stdin = strings.NewReader(m.content)
			if err := c.Run(); err == nil {
				m.toast = "✅ Copied"
			} else {
				m.toast = "❌ Copy Failed"
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		headerHeight := 3
		footerHeight := 3
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.SetContent(m.content)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m reportViewerModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("42")).
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(lipgloss.Color("42")).
		Padding(0, 1).
		Width(m.viewport.Width)

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(lipgloss.Color("241")).
		Padding(0, 1).
		Width(m.viewport.Width)

	header := headerStyle.Render("✨ GITHUB REPORT GENERATED")
	
	footerText := fmt.Sprintf("%3.f%% • [c] copy • [p] print • [r] regen • [q] quit", m.viewport.ScrollPercent()*100)
	if m.toast != "" {
		footerText = fmt.Sprintf("%3.f%% • %s • [c] copy • [p] print • [r] regen • [q] quit", m.viewport.ScrollPercent()*100, m.toast)
	}
	footer := footerStyle.Render(footerText)

	return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)
}

func main() {
	// Simple version check before anything else
	for _, arg := range os.Args {
		if arg == "-v" || arg == "--version" || arg == "version" {
			printVersion()
			return
		}
	}

	checkForUpdates()

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

	items := []list.Item{
		menuItem{title: "🚀 Report", desc: "Generate a new AI summary of GitHub commits", action: "Report"},
		menuItem{title: "⚙️  Setting", desc: "Configure your API keys (Groq & Gemini)", action: "Setting"},
		menuItem{title: "❌ Exit", desc: "Quit the application", action: "Exit"},
	}

	for {
		m := menuModel{list: list.New(items, list.NewDefaultDelegate(), 50, 14)}
		m.list.Title = "Main Menu"
		m.list.SetShowStatusBar(false)
		m.list.SetFilteringEnabled(false)
		m.list.SetShowHelp(false)

		p := tea.NewProgram(m)
		model, err := p.Run()
		if err != nil {
			fmt.Printf("Error running menu: %v", err)
			os.Exit(1)
		}

		finalModel := model.(menuModel)
		if finalModel.quitting || finalModel.choice == "Exit" {
			break
		}

		if finalModel.choice == "Update" && latestRelease != nil {
			doUpdate(latestRelease)
		} else if finalModel.choice == "Report" {
			runReport(confPath)
		} else if finalModel.choice == "Setting" {
			runSettings(confPath)
		}
	}
}

func runSettings(path string) {
	for {
		gk := os.Getenv("GROQ_API_KEY")
		gm := os.Getenv("GEMINI_API_KEY")

		gkS := color.RedString("Empty")
		if gk != "" {
			gkS = color.GreenString("Configured")
		}
		gmS := color.RedString("Empty")
		if gm != "" {
			gmS = color.GreenString("Configured")
		}

		ws := os.Getenv("WORK_START")
		we := os.Getenv("WORK_END")
		if ws == "" { ws = "9" }
		if we == "" { we = "17" }

		items := []list.Item{
			menuItem{title: "Groq API Key", desc: "Status: " + gkS, action: "Groq"},
			menuItem{title: "Gemini API Key", desc: "Status: " + gmS, action: "Gemini"},
			menuItem{title: "🕒 Work Start", desc: "Currently: " + ws + ":00", action: "WorkStart"},
			menuItem{title: "🕔 Work End", desc: "Currently: " + we + ":00", action: "WorkEnd"},
			menuItem{title: "⬅️  Back", desc: "Return to Main Menu", action: "Back"},
		}

		m := menuModel{list: list.New(items, list.NewDefaultDelegate(), 50, 14)}
		m.list.Title = "Settings"
		m.list.SetShowStatusBar(false)
		m.list.SetFilteringEnabled(false)
		m.list.SetShowHelp(false)

		p := tea.NewProgram(m)
		model, err := p.Run()
		if err != nil {
			break
		}

		finalModel := model.(menuModel)
		if finalModel.quitting || finalModel.choice == "Back" {
			break
		}

		var res string
		if finalModel.choice == "Groq" {
			huh.NewInput().Title("Enter Groq API Key").EchoMode(huh.EchoModePassword).Value(&res).Run()
			if res != "" {
				os.Setenv("GROQ_API_KEY", res)
			}
		} else if finalModel.choice == "Gemini" {
			huh.NewInput().Title("Enter Gemini API Key").EchoMode(huh.EchoModePassword).Value(&res).Run()
			if res != "" {
				os.Setenv("GEMINI_API_KEY", res)
			}
		} else if finalModel.choice == "WorkStart" {
			huh.NewInput().Title("Enter Work Start Hour (0-23)").Value(&res).Run()
			if res != "" {
				os.Setenv("WORK_START", res)
			}
		} else if finalModel.choice == "WorkEnd" {
			huh.NewInput().Title("Enter Work End Hour (0-23)").Value(&res).Run()
			if res != "" {
				os.Setenv("WORK_END", res)
			}
		}

		content := fmt.Sprintf("GROQ_API_KEY=%s\nGEMINI_API_KEY=%s\nWORK_START=%s\nWORK_END=%s\n", 
			os.Getenv("GROQ_API_KEY"), os.Getenv("GEMINI_API_KEY"), 
			os.Getenv("WORK_START"), os.Getenv("WORK_END"))
		
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

func checkForUpdates() {
	if Version == "dev" {
		return // Don't check for updates in dev builds
	}

	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	_ = spin.Color("cyan", "bold")
	spin.Suffix = color.HiBlackString(" Checking for updates...")
	spin.Start()

	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		spin.Stop()
		return
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{Source: source})
	if err != nil {
		spin.Stop()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := selfupdate.ParseSlug("sakatimuna7/github-report")
	latest, found, err := updater.DetectLatest(ctx, repo)
	spin.Stop()
	if err != nil || !found {
		return
	}

	if latest.LessOrEqual(Version) {
		return
	}

	latestRelease = latest
}

func doUpdate(latest *selfupdate.Release) {
	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	_ = spin.Color("cyan", "bold")

	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{Source: source})
	if err != nil {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		color.Red("❌ Failed to get executable path: %v", err)
		return
	}

	// Wrapper for progress bar
	onProgress := func(downloaded, total int64) {
		if total <= 0 {
			return
		}
		pct := float64(downloaded) / float64(total) * 100
		barLen := 30
		filledLen := int(float64(barLen) * pct / 100)
		if filledLen > barLen {
			filledLen = barLen
		}
		bar := strings.Repeat("█", filledLen) + strings.Repeat("░", barLen-filledLen)
		fmt.Printf("\r%s [%s] %.1f%% (%s/%s)", 
			color.HiBlackString(" Downloading:"), 
			color.CyanString(bar), 
			pct, 
			humanize.Bytes(uint64(downloaded)), 
			humanize.Bytes(uint64(total)))
	}

	// Update the updater with a progress-aware source
	updater, _ = selfupdate.NewUpdater(selfupdate.Config{
		Source: &progressSource{
			Source:     source,
			onProgress: onProgress,
		},
	})

	// Use a longer timeout for the actual download and installation
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer updateCancel()

	if err := updater.UpdateTo(updateCtx, latest, exe); err != nil {
		fmt.Println() // New line after progress bar
		if strings.Contains(err.Error(), "permission denied") {
			color.Red("❌ Update failed: Permission denied.")
			color.Yellow("Tip: Try running with 'sudo' to allow updating to %s", exe)
			color.HiBlackString("Example: sudo ghreport")
		} else {
			color.Red("❌ Update failed: %v", err)
		}
		return
	}
	fmt.Println() // New line after progress bar

	color.Green("✅ Successfully updated to %s!", latest.Version())
	color.Yellow("Please restart the application to use the new version.")
	os.Exit(0)
}

func runReport(confPath string) {
	fs := flag.NewFlagSet("ghreport", flag.ContinueOnError)
	owner := fs.String("owner", "", "")
	repo := fs.String("repo", "", "")
	branch := fs.String("branch", "", "")
	lim := fs.Int("limit", 0, "")
	tok := fs.String("token", os.Getenv("GITHUB_TOKEN"), "")
	gk := fs.String("groq-key", os.Getenv("GROQ_API_KEY"), "")
	gm := fs.String("gemini-key", os.Getenv("GEMINI_API_KEY"), "")
	mod := fs.String("ai", "gemini-flash", "")
	
	_ = fs.Parse(os.Args[1:])

	if *owner == "" || *repo == "" || (fs.NArg() > 0 && fs.Arg(0) == ".") {
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

	var dr, fr, ctxN string

	// We wrap everything from the form down in a loop so they can regenerate
	for {
		var dOpts []huh.Option[string]
		now := time.Now()
		for i := 0; i < 7; i++ {
			str := now.AddDate(0, 0, -i).Format("02/01/2006")
			dOpts = append(dOpts, huh.NewOption(str, str))
		}
		dOpts = append(dOpts, huh.NewOption("Custom Range", "Custom Range"))

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("AI Model").
					Options(
						huh.NewOption("gemini-flash", "gemini-flash"),
						huh.NewOption("gemini-flash-lite", "gemini-flash-lite"),
						huh.NewOption("groq-llama", "groq-llama"),
						huh.NewOption("groq-mixtral", "groq-mixtral"),
						huh.NewOption("groq-gpt", "groq-gpt"),
					).
					Value(mod),
				huh.NewSelect[string]().
					Title("Date Period").
					Options(dOpts...).
					Value(&dr),
				huh.NewSelect[string]().
					Title("Focus").
					Options(
						huh.NewOption("1. Semua", "1. Semua"),
						huh.NewOption("2. Summary", "2. Summary"),
						huh.NewOption("3. Changes", "3. Changes"),
						huh.NewOption("4. Modules", "4. Modules"),
						huh.NewOption("5. Authors", "5. Authors"),
						huh.NewOption("6. Recs", "6. Recs"),
					).
					Value(&fr),
				huh.NewInput().
					Title("Context (optional)").
					Value(&ctxN),
			),
		)

		err := form.Run()
		if err != nil {
			return
		}

		var s, u time.Time
		if dr == "Custom Range" {
			var since, until string
			err := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().Title("Since (YYYY-MM-DD)").Value(&since),
					huh.NewInput().Title("Until (YYYY-MM-DD, Optional)").Value(&until),
				),
			).Run()
			if err != nil {
				return
			}
			s, _ = time.Parse("2006-01-02", since)
			if until != "" {
				u, _ = time.Parse("2006-01-02", until)
			} else {
				u = time.Now()
			}
		} else {
			sd, _ := time.Parse("02/01/2006", dr)
			s = time.Date(sd.Year(), sd.Month(), sd.Day(), 0, 0, 0, 0, sd.Location())
			u = time.Date(sd.Year(), sd.Month(), sd.Day(), 23, 59, 59, 0, sd.Location())
		}

	h, _ := os.UserHomeDir()
	cache := h + "/.ghreport_cache"
	_ = os.MkdirAll(cache, 0755)
	cc := pipeline.NewFileCache(cache + "/" + fmt.Sprintf("%s_%s_%s_%s_chunks.json", *owner, *repo, *branch, s.Format("2006-01-02")))

	c := context.Background()
	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	_ = spin.Color("cyan", "bold")
	spin.Suffix = color.HiBlackString(" Fetching GitHub Data...")
	spin.Start()
	ws, we := 9, 17
	if s := os.Getenv("WORK_START"); s != "" {
		fmt.Sscanf(s, "%d", &ws)
	}
	if s := os.Getenv("WORK_END"); s != "" {
		fmt.Sscanf(s, "%d", &we)
	}

	raw, stats, err := github.NewClient(*tok).GetReportData(c, *owner, *repo, *branch, *lim, s, u, ws, we)
	spin.Stop()
	if err != nil {
		fmt.Printf(color.RedString("Error: %v\n", err))
		return
	}

	// Confirmation Step
	var proceed bool
	
	columns := []table.Column{
		{Title: "Total Commits", Width: 15},
		{Title: "Features", Width: 10},
		{Title: "Fixes", Width: 10},
		{Title: "Overtime", Width: 10},
	}

	rows := []table.Row{
		{
			fmt.Sprintf("%d", stats.Total),
			fmt.Sprintf("%d", stats.Features),
			fmt.Sprintf("%d", stats.Fixes),
			fmt.Sprintf("%d", stats.Overtime),
		},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(false),
		table.WithHeight(3),
	)

	purple := lipgloss.Color("99")
	st := table.DefaultStyles()
	st.Header = st.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(purple).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		BorderTop(true).
		Foreground(purple).
		Bold(true)
	st.Cell = st.Cell.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(purple).
		BorderLeft(true).
		BorderRight(true).
		BorderBottom(true)
	st.Selected = lipgloss.NewStyle()
	t.SetStyles(st)

	tableStyle := lipgloss.NewStyle().
		MarginBottom(1)

	summary := tableStyle.Render(t.View())

	// Selection Table
	selColumns := []table.Column{
		{Title: "AI Model", Width: 15},
		{Title: "Period", Width: 15},
		{Title: "Focus", Width: 15},
	}
	selRows := []table.Row{
		{*mod, dr, fr},
	}
	stbl := table.New(
		table.WithColumns(selColumns),
		table.WithRows(selRows),
		table.WithFocused(false),
		table.WithHeight(3),
	)
	stbl.SetStyles(st)
	selectionSummary := tableStyle.Render(stbl.View())

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Review Selections").
				Description(selectionSummary),
			huh.NewNote().
				Title("Commit Statistics").
				Description(summary),
			huh.NewConfirm().
				Title("Proceed to generate AI report?").
				Affirmative("Yes, execute").
				Negative("No, go back").
				Value(&proceed),
		),
	).Run()

	if err != nil || !proceed {
		continue
	}

	fmt.Println()
	color.Cyan("╭────────────────────────────────────────╮")
	color.Cyan("│ 🚀 GENERATING REPORT                   │")
	color.Cyan("╰────────────────────────────────────────╯")

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

	spin.Suffix = color.HiBlackString(" MAP Phase (Parallel Analysis)...")
	spin.Restart()
	mRes := pool.Run(c, chunks, func(ctx context.Context, d string) (string, error) { return fb(mm, pipeline.MapSysPrompt, d) })
	spin.Stop()
	sums, _ := pipeline.CollectSuccessful(mRes)
	_ = cc.Flush()

	spin.Suffix = color.HiBlackString(" REDUCE Phase (Merging Insights)...")
	spin.Restart()
	merged, _ := fb(rm, pipeline.ReduceSysPrompt, strings.Join(sums, "\n---\n"))

	fi := "Full report."
	if fr != "1. Semua" {
		fi = fr
	}
	sp := fmt.Sprintf("Role: SE\nTask: Format report\nLanguage: Bahasa Indonesia\nFocus: %s\nContext: %s\nRules: clean headings, bullet points, concise, NO bold tags (**text**)", fi, ctxN)

	report, _ := fb(mm, sp, merged)
	spin.Stop()

	reportContent := fmt.Sprintf("%s\n\nUsage: %d Prompt | %d Completion | %d Total Tokens\n", report, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)

	p := tea.NewProgram(
		reportViewerModel{content: reportContent},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	model, err := p.Run()
	if err != nil {
		fmt.Printf("Error rendering report: %v\n", err)
		return
	}

	finalModel := model.(reportViewerModel)
	if finalModel.action == "regen" {
		continue
	} else if finalModel.action == "print" {
		fmt.Println("\n" + reportContent + "\n")
		return
	} else {
		return
	}
	}
}

type progressReader struct {
	io.ReadCloser
	total      int64
	downloaded int64
	onProgress func(downloaded, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.ReadCloser.Read(p)
	pr.downloaded += int64(n)
	if pr.onProgress != nil {
		pr.onProgress(pr.downloaded, pr.total)
	}
	return n, err
}

type progressSource struct {
	selfupdate.Source
	onProgress func(downloaded, total int64)
}

func (ps *progressSource) DownloadReleaseAsset(ctx context.Context, rel *selfupdate.Release, assetID int64) (io.ReadCloser, error) {
	rc, err := ps.Source.DownloadReleaseAsset(ctx, rel, assetID)
	if err != nil {
		return nil, err
	}
	var size int64
	if assetID == rel.AssetID {
		size = int64(rel.AssetByteSize)
	}
	return &progressReader{
		ReadCloser: rc,
		total:      size,
		onProgress: ps.onProgress,
	}, nil
}

