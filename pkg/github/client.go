package github

import (
	"context"
	"fmt"
	"strings"
	"time"
	"regexp"
	"strconv"
	"sync"

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
			// If branch not found, try empty SHA (GitHub will use default branch)
			if opts.SHA != "" {
				opts.SHA = ""
				continue
			}
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

	// Phase 2: Pre-fetch Referenced Issues & PRs for deep context
	issueRegex := regexp.MustCompile(`(?i)(?:fixes|resolves|closes|refs)?\s*#(\d+)`)
	issueMap := make(map[string]*github.Issue)
	uniqueIDs := make(map[int]bool)

	for _, commit := range allCommits {
		matches := issueRegex.FindAllStringSubmatch(commit.GetCommit().GetMessage(), -1)
		for _, m := range matches {
			if len(m) > 1 {
				id, _ := strconv.Atoi(m[1])
				uniqueIDs[id] = true
			}
		}
	}

	for id := range uniqueIDs {
		issue, _, err := c.client.Issues.Get(ctx, owner, repo, id)
		if err == nil && issue != nil {
			issueMap[strconv.Itoa(id)] = issue
		}
	}

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
		
		// Enrich message with issue titles if found
		enrichedMsg := shortMsg
		matches := issueRegex.FindAllStringSubmatch(fullMsg, -1)
		for _, m := range matches {
			if len(m) > 1 {
				if issue, ok := issueMap[m[1]]; ok {
					enrichedMsg += fmt.Sprintf(" (Issue Context: %s)", issue.GetTitle())
				}
			}
		}

		sb.WriteString(fmt.Sprintf("- %s (by %s)\n", enrichedMsg, author))
	}

	if len(issueMap) > 0 {
		sb.WriteString("\n\n[Deep Context: Referenced Issues & PRs]\n")
		for id, issue := range issueMap {
			body := issue.GetBody()
			if len(body) > 300 {
				body = body[:300] + "..."
			}
			body = strings.ReplaceAll(body, "\n", " ")
			sb.WriteString(fmt.Sprintf("- #%s [%s]: %s\n  Summary: %s\n", id, issue.GetState(), issue.GetTitle(), body))
		}
	}

	return sb.String(), stats, nil
}
func (c *Client) GetUserLogin(ctx context.Context) (string, error) {
	u, _, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return "", err
	}
	return u.GetLogin(), nil
}

type DashboardData struct {
	Languages     map[string]int
	Contributions []int // Last 30 days
}

func (c *Client) GetDashboardData(ctx context.Context, username string) (DashboardData, error) {
	data := DashboardData{
		Languages:     make(map[string]int),
		Contributions: make([]int, 30),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// 1. Languages (Parallel fetch per repo)
	wg.Add(1)
	go func() {
		defer wg.Done()
		repos, _, err := c.client.Repositories.List(ctx, "", &github.RepositoryListOptions{
			ListOptions: github.ListOptions{PerPage: 20},
		})
		if err == nil {
			var repoWg sync.WaitGroup
			for _, r := range repos {
				if r.GetOwner() == nil { continue }
				repoWg.Add(1)
				go func(owner, name string) {
					defer repoWg.Done()
					langs, _, err := c.client.Repositories.ListLanguages(ctx, owner, name)
					if err == nil {
						mu.Lock()
						for l, bytes := range langs {
							data.Languages[l] += bytes
						}
						mu.Unlock()
					}
				}(r.GetOwner().GetLogin(), r.GetName())
			}
			repoWg.Wait()
		}
	}()

	// 2. Contributions
	wg.Add(1)
	go func() {
		defer wg.Done()
		events, _, err := c.client.Activity.ListEventsPerformedByUser(ctx, username, false, &github.ListOptions{PerPage: 100})
		if err == nil {
			now := time.Now()
			for _, e := range events {
				createdAt := e.GetCreatedAt().Time
				diff := int(now.Sub(createdAt).Hours() / 24)
				dayIdx := 29 - diff
				
				if dayIdx >= 0 && dayIdx < 30 {
					switch e.GetType() {
					case "PushEvent":
						payload, _ := e.ParsePayload()
						if push, ok := payload.(*github.PushEvent); ok {
							mu.Lock()
							data.Contributions[dayIdx] += push.GetSize()
							mu.Unlock()
						}
					case "PullRequestEvent", "IssuesEvent", "CreateEvent":
						mu.Lock()
						data.Contributions[dayIdx]++
						mu.Unlock()
					}
				}
			}
		}
	}()

	wg.Wait()
	return data, nil
}
