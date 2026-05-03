package updater

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
)

func CheckForUpdates(version string) *selfupdate.Release {
	if version == "dev" {
		return nil
	}

	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	_ = spin.Color("cyan", "bold")
	spin.Suffix = color.HiBlackString(" Checking for updates...")
	spin.Start()

	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		spin.Stop()
		return nil
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{Source: source})
	if err != nil {
		spin.Stop()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := selfupdate.ParseSlug("sakatimuna7/github-report")
	latest, found, err := updater.DetectLatest(ctx, repo)
	spin.Stop()
	if err != nil || !found {
		return nil
	}

	if latest.LessOrEqual(version) {
		return nil
	}

	return latest
}

func DoUpdate(latest *selfupdate.Release) {
	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	_ = spin.Color("cyan", "bold")

	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		color.Red("❌ Failed to get executable path: %v", err)
		return
	}

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

	updater, _ := selfupdate.NewUpdater(selfupdate.Config{
		Source: &progressSource{
			Source:     source,
			onProgress: onProgress,
		},
	})

	updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer updateCancel()

	if err := updater.UpdateTo(updateCtx, latest, exe); err != nil {
		fmt.Println()
		if strings.Contains(err.Error(), "permission denied") {
			color.Red("❌ Update failed: Permission denied.")
			color.Yellow("Tip: Try running with 'sudo'")
		} else {
			color.Red("❌ Update failed: %v", err)
		}
		return
	}
	fmt.Println()

	color.Green("✅ Successfully updated to %s!", latest.Version())
	color.Yellow("Please restart the application to use the new version.")
	os.Exit(0)
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
