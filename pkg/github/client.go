package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

type Client struct {
	client *github.Client
}

func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		client: github.NewClient(tc),
	}
}

type CommitStats struct {
	Total    int
	Features int
	Fixes    int
	Overtime int
}

func (c *Client) GetReportData(ctx context.Context, owner, repo, branch string, limit int, since, until time.Time, workStart, workEnd int) (string, CommitStats, error) {
	var allCommits []*github.RepositoryCommit
	
	opts := &github.CommitsListOptions{
		SHA: branch,
		ListOptions: github.ListOptions{
			PerPage: 100, // Max per page for efficiency
		},
	}

	if !since.IsZero() {
		opts.Since = since
	}
	if !until.IsZero() {
		opts.Until = until
	}

	for {
		commits, resp, err := c.client.Repositories.ListCommits(ctx, owner, repo, opts)
		if err != nil {
			return "", CommitStats{}, fmt.Errorf("error fetching commits: %w", err)
		}
		
		allCommits = append(allCommits, commits...)
		
		// Break if we reached the limit (if limit > 0)
		if limit > 0 && len(allCommits) >= limit {
			allCommits = allCommits[:limit]
			break
		}
		
		// Break if no more pages
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	stats := CommitStats{Total: len(allCommits)}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Repo: %s/%s | Branch: %s\n\n", owner, repo, branch))
	sb.WriteString(fmt.Sprintf("Total activity fetched: %d commits\n", len(allCommits)))

	lastDate := ""
	for _, commit := range allCommits {
		fullMsg := commit.GetCommit().GetMessage()
		lowerMsg := strings.ToLower(fullMsg)
		
		// Basic conventional commit detection
		if strings.HasPrefix(lowerMsg, "feat") {
			stats.Features++
		} else if strings.HasPrefix(lowerMsg, "fix") {
			stats.Fixes++
		}
		
		// Overtime check: Outside workStart to workEnd
		date := commit.GetCommit().GetAuthor().GetDate()
		hour := date.Hour()
		if hour >= workEnd || hour < workStart {
			stats.Overtime++
		}

		// Only take the first line of the commit message (subject)
		shortMsg := strings.Split(fullMsg, "\n")[0]
		
		author := commit.GetCommit().GetAuthor().GetName()
		// Format date to YYYY-MM-DD
		fullDate := commit.GetCommit().GetAuthor().GetDate().Format("2006-01-02")
		
		if fullDate != lastDate {
			sb.WriteString(fmt.Sprintf("\n[%s]\n", fullDate))
			lastDate = fullDate
		}
		
		sb.WriteString(fmt.Sprintf("- %s (by %s)\n", shortMsg, author))
	}

	return sb.String(), stats, nil
}
