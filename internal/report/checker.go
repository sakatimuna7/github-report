package report

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
)

func CheckGitHubCLI() {
	// 1. Check if gh is installed
	_, err := exec.LookPath("gh")
	if err != nil {
		color.Yellow("⚠️  GitHub CLI (gh) tidak ditemukan di sistem kamu.")
		
		var install bool
		err := huh.NewConfirm().
			Title("Mau install GitHub CLI (gh) sekarang?").
			Affirmative("Ya, install").
			Negative("Gak usah, nanti aja").
			Value(&install).
			Run()
		
		if err != nil || !install {
			color.Red("❌ GitHub CLI diperlukan untuk menjalankan aplikasi ini.")
			os.Exit(1)
		}

		installGH()
	}

	// 2. Check if logged in
	out, err := exec.Command("gh", "auth", "status").CombinedOutput()
	status := string(out)
	
	// gh auth status returns non-zero if not logged in or some error
	if err != nil || strings.Contains(status, "Logged in to none") || strings.Contains(status, "You are not logged into any GitHub hosts") {
		color.Yellow("⚠️  Kamu belum login ke GitHub CLI.")
		
		var login bool
		err := huh.NewConfirm().
			Title("Mau login ke GitHub sekarang?").
			Affirmative("Ya, login").
			Negative("Gak usah, nanti aja").
			Value(&login).
			Run()
		
		if err != nil || !login {
			color.Red("❌ Kamu harus login untuk mengambil data repository.")
			os.Exit(1)
		}

		// Run interactive login
		cmd := exec.Command("gh", "auth", "login")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		
		// Check again after login attempt
		out, err = exec.Command("gh", "auth", "status").CombinedOutput()
		if err != nil || strings.Contains(string(out), "Logged in to none") {
			color.Red("❌ Login gagal atau dibatalkan.")
			os.Exit(1)
		}
		color.Green("✅ Login berhasil!")
	}
}

func installGH() {
	var cmd *exec.Cmd
	osType := runtime.GOOS

	color.Cyan("🚀 Sedang memproses instalasi untuk %s...", osType)

	switch osType {
	case "darwin":
		// Check if brew is installed
		_, err := exec.LookPath("brew")
		if err != nil {
			color.Red("❌ Homebrew tidak ditemukan. Silakan install gh manual: https://cli.github.com/")
			os.Exit(1)
		}
		cmd = exec.Command("brew", "install", "gh")
	case "linux":
		color.Yellow("ℹ️  Silakan install gh menggunakan package manager kamu (apt, dnf, pacman, dll).")
		color.Cyan("Contoh (Ubuntu/Debian): sudo apt install gh")
		os.Exit(0)
	case "windows":
		cmd = exec.Command("winget", "install", "--id", "GitHub.cli")
	default:
		color.Red("❌ OS tidak didukung untuk auto-install. Silakan install gh manual: https://cli.github.com/")
		os.Exit(1)
	}

	if cmd != nil {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			color.Red("❌ Gagal menginstall GitHub CLI: %v", err)
			os.Exit(1)
		}
		color.Green("✅ GitHub CLI berhasil diinstall!")
	}
}
