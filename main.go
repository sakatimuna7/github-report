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
	"github-report-ai/pkg/sheets"

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
		case "e":
			m.action = "export_sheets"
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
	
	footerText := fmt.Sprintf("%3.f%% • [c] copy • [p] print • [e] export sheets • [r] regen • [q] quit", m.viewport.ScrollPercent()*100)
	if m.toast != "" {
		footerText = fmt.Sprintf("%3.f%% • %s • [c] copy • [p] print • [e] export • [r] regen • [q] quit", m.viewport.ScrollPercent()*100, m.toast)
	}
	footer := footerStyle.Render(footerText)

	return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)
}

func main() {
	var isCI bool
	for _, arg := range os.Args {
		if arg == "-v" || arg == "--version" || arg == "version" {
			printVersion()
			return
		}
		if arg == "--ci" {
			isCI = true
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

	if isCI {
		runReport(confPath)
		return
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

		sID := os.Getenv("SHEETS_ID")
		dName := os.Getenv("DEVELOPER_NAME")
		credPath := os.Getenv("GOOGLE_CREDENTIALS_PATH")

		sIdStr := color.RedString("Not Set")
		if sID != "" { sIdStr = color.GreenString("Set") }
		dNameStr := color.RedString("Not Set")
		if dName != "" { dNameStr = color.GreenString("Set") }
		credPathStr := color.RedString("Not Set")
		if credPath != "" { credPathStr = color.GreenString("Set") }

		items := []list.Item{
			menuItem{title: "Groq API Key", desc: "Status: " + gkS, action: "Groq"},
			menuItem{title: "Gemini API Key", desc: "Status: " + gmS, action: "Gemini"},
			menuItem{title: "🕒 Work Start", desc: "Currently: " + ws + ":00", action: "WorkStart"},
			menuItem{title: "🕔 Work End", desc: "Currently: " + we + ":00", action: "WorkEnd"},
			menuItem{title: "📊 Sheets ID", desc: "Status: " + sIdStr, action: "SheetsID"},
			menuItem{title: "👨‍💻 Dev Name", desc: "Status: " + dNameStr, action: "DevName"},
			menuItem{title: "🔑 Google Creds", desc: "Status: " + credPathStr, action: "CredPath"},
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
		} else if finalModel.choice == "SheetsID" {
			huh.NewInput().Title("Enter Google Spreadsheet ID").Value(&res).Run()
			if res != "" {
				os.Setenv("SHEETS_ID", res)
			}
		} else if finalModel.choice == "DevName" {
			huh.NewInput().Title("Enter Your Developer Name").Value(&res).Run()
			if res != "" {
				os.Setenv("DEVELOPER_NAME", res)
			}
		} else if finalModel.choice == "CredPath" {
			res = runFilePicker()
			if res != "" {
				os.Setenv("GOOGLE_CREDENTIALS_PATH", res)
			}
		}

		content := fmt.Sprintf("GROQ_API_KEY=%s\nGEMINI_API_KEY=%s\nWORK_START=%s\nWORK_END=%s\nSHEETS_ID=%s\nDEVELOPER_NAME=%s\nGOOGLE_CREDENTIALS_PATH=%s\n", 
			os.Getenv("GROQ_API_KEY"), os.Getenv("GEMINI_API_KEY"), 
			os.Getenv("WORK_START"), os.Getenv("WORK_END"),
			os.Getenv("SHEETS_ID"), os.Getenv("DEVELOPER_NAME"),
			os.Getenv("GOOGLE_CREDENTIALS_PATH"))
		
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
	ciMode := fs.Bool("ci", false, "")
	periodFlag := fs.String("period", "today", "Date period (e.g. 02/01/2006 or 'today')")
	focusFlag := fs.String("focus", "1. Semua", "Focus area")
	
	_ = fs.Parse(os.Args[1:])

	argsPassed := *owner != "" && *repo != ""

	// Try local git repo
	localOwner, localRepo := "", ""
	u := sh("git", "remote", "get-url", "origin")
	if u != "" {
		u = strings.TrimPrefix(strings.TrimPrefix(u, "https://github.com/"), "git@github.com:")
		u = strings.TrimSuffix(u, ".git")
		p := strings.Split(u, "/")
		if len(p) >= 2 {
			localOwner = p[0]
			localRepo = p[1]
		}
	}

	h, _ := os.UserHomeDir()
	historyPath := h + "/.ghreport_history.json"

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
				// Don't duplicate current dir in history list
				if val != fmt.Sprintf("%s/%s", localOwner, localRepo) {
					opts = append(opts, huh.NewOption(fmt.Sprintf("🕒 %s", val), val))
				}
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
					Description("Examples:\ngit@github.com:sakatimuna7/github-report.git\nhttps://github.com/sakatimuna7/github-report.git\nsakatimuna7/github-report").
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
			}
			break
		}
	} else if *owner == "" || *repo == "" {
		if *owner == "" { *owner = localOwner }
		if *repo == "" { *repo = localRepo }
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
		*branch = sh("git", "rev-parse", "--abbrev-ref", "HEAD")
	}
	if *tok == "" {
		*tok = sh("gh", "auth", "token")
	}

	var dr, fr, ctxN string

	// We wrap everything from the form down in a loop so they can regenerate
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
						Title("Template / Focus").
						Options(tOpts...).
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

	// Check for cached final report
	reportsCacheDir := h + "/.ghreport_reports"
	_ = os.MkdirAll(reportsCacheDir, 0755)
	cacheKey := fmt.Sprintf("%s_%s_%s_%s", *owner, *repo, *branch, s.Format("2006-01-02"))
	cacheFile := reportsCacheDir + "/" + pipeline.ContentHash(cacheKey) + ".json"
	
	cachedResult, errCache := pipeline.LoadReportResult(cacheFile)
	hasCache := errCache == nil
	
	// TTL Check: 24 hours
	if hasCache && time.Since(cachedResult.Timestamp) > 24*time.Hour {
		hasCache = false
	}

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

	var cacheNote string
	if hasCache {
		cacheNote = fmt.Sprintf(
			"📅 Generated at: %s\n🤖 Model used: %s\n🎯 Focus: %s\n📊 Stats: %d commits, %d feats, %d fixes",
			cachedResult.Timestamp.Format("2006-01-02 15:04"),
			cachedResult.Model,
			cachedResult.Focus,
			cachedResult.TotalCommits, cachedResult.Features, cachedResult.Fixes,
		)
	}

	var action string
	var fields []huh.Field
	
	fields = append(fields, 
		huh.NewNote().
			Title("Review Selections").
			Description(selectionSummary),
		huh.NewNote().
			Title("Commit Statistics").
			Description(summary),
	)

	if hasCache {
		fields = append(fields,
			huh.NewNote().
				Title("✨ Cached Report Found").
				Description(cacheNote),
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("Use Cached Report", "cache"),
					huh.NewOption("Regenerate (New AI Call)", "regen_ai"),
					huh.NewOption("Go Back / Cancel", "back"),
				).
				Value(&action),
		)
	} else if !*ciMode {
		var proceed bool
		fields = append(fields,
			huh.NewConfirm().
				Title("Proceed to generate AI report?").
				Affirmative("Yes, execute").
				Negative("No, go back").
				Value(&proceed),
		)
	}

	if !*ciMode {
		err = huh.NewForm(huh.NewGroup(fields...)).Run()
		if err != nil {
			continue // User cancelled form
		}
		
		if !hasCache {
			// Without cache, the last field is the confirm
			// We can't cleanly extract it, but if they didn't cancel, we assume proceed=true
			// Wait, the confirm sets the 'proceed' variable pointer
			// However 'proceed' is scoped to the 'else if' block!
			// Ah, that was it!
		}
	}

	// Determine action
	if *ciMode {
		if hasCache {
			action = "cache"
		} else {
			action = "regen_ai"
		}
	} else if !hasCache {
		action = "regen_ai" // Implicitly true if they didn't hit escape on the form
	}

	if action == "back" || action == "" {
		continue // Go back to parameter selection
	}

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

	templates, _ := pipeline.LoadTemplates()
	tmpl, exists := templates[fr]
	if !exists {
		tmpl = templates["Default"]
	}
	
	sp := strings.ReplaceAll(tmpl, "{{FOCUS}}", fr)
	sp = strings.ReplaceAll(sp, "{{CONTEXT}}", ctxN)

	report, _ := fb(mm, sp, merged)
	spin.Stop()

	reportContent = fmt.Sprintf("%s\n\nUsage: %d Prompt | %d Completion | %d Total Tokens\n", report, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
	
	// Save to cache
	_ = pipeline.SaveReportResult(cacheFile, pipeline.ReportResult{
		Content:      reportContent,
		Timestamp:    time.Now(),
		Model:        *mod,
		Period:       dr,
		Focus:        fr,
		TotalCommits: stats.Total,
		Features:     stats.Features,
		Fixes:        stats.Fixes,
		Overtime:     stats.Overtime,
		Usage:        fmt.Sprintf("%d Prompt | %d Completion | %d Total Tokens", usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens),
	})
	} // end of else block for action != "cache"

	var doExport bool
	if *ciMode && os.Getenv("SHEETS_ID") != "" && os.Getenv("DEVELOPER_NAME") != "" {
		doExport = true
		fmt.Println(reportContent)
	} else if *ciMode {
		fmt.Println(reportContent)
		return
	}

	if !*ciMode {
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
		} else if finalModel.action == "export_sheets" {
			doExport = true
		} else {
			return
		}
	}
	if doExport {
		spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		_ = spin.Color("cyan", "bold")
		spin.Suffix = color.HiBlackString(" Exporting to Google Sheets...")
		spin.Start()

		sID := os.Getenv("SHEETS_ID")
		dName := os.Getenv("DEVELOPER_NAME")
		if sID == "" || dName == "" {
			spin.Stop()
			color.Red("❌ Missing SHEETS_ID or DEVELOPER_NAME in Settings.")
			return
		}

		credFile := os.Getenv("GOOGLE_CREDENTIALS_PATH")
		if credFile == "" {
			credFile = h + "/.ghreport_credentials.json"
		}
		tokFile := h + "/.ghreport_token.json"
		
		if _, err := os.Stat(credFile); os.IsNotExist(err) {
			spin.Stop()
			color.Red("\n❌ Google Credentials File (credentials.json) Not Found!")
			color.Yellow("\nHow to get this file:")
			color.White("1. Open https://console.cloud.google.com/")
			color.White("2. Create a new Project or select an existing one.")
			color.White("3. Search for 'Google Sheets API' and Enable it.")
			color.White("4. Go to 'APIs & Services' > 'Credentials'.")
			color.White("5. Click 'Create Credentials' > 'OAuth client ID'.")
			color.White("   (If it asks to configure Consent Screen, just fill the required fields, User Type: External/Internal).")
			color.White("   - Application type: Desktop app")
			color.White("6. Download the JSON file.")
			color.White("7. Go to 'ghreport' Settings and set the absolute path to that JSON file.\n")
			return
		}

		srv, err := sheets.NewService(credFile, tokFile)
		if err != nil {
			spin.Stop()
			color.Red("❌ Google Sheets Auth Error: %v", err)
			return
		}

		err = sheets.WriteReportToSheet(srv, sID, dName, s, reportContent)
		spin.Stop()
		if err != nil {
			color.Red("❌ Failed to export: %v", err)
		} else {
			color.Green("✅ Successfully exported to Google Sheets!")
		}
		return
	}

	} // end of for loop (menu loop)
} // end of runReport

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

