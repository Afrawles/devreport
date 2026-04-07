package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Afrawles/devreport/internal/report"
)

type GitHubSource struct {
	Client *Client
}

func NewGitHubSource(token string, orgs []string, username string, includeReviewedPRs, includeAssignedIssues bool) *GitHubSource {
	return &GitHubSource{
		Client: NewClient(token, orgs, username, includeReviewedPRs, includeAssignedIssues),
	}
}

var _ report.ActivitySource = (*GitHubSource)(nil)

func (g *GitHubSource) Name() string {
	return "GitHub"
}

func (g *GitHubSource) HealthCheck() error {
	return g.Client.HealthCheck()
}

func (g *GitHubSource) FetchTasks(user string, start, end time.Time) ([]report.Task, error) {
	ctx := context.Background()
	var allTasks []report.Task

	// Fetch commits
	commits, err := g.Client.FetchCommits(ctx, start, end)
	if err != nil {
		fmt.Printf("Error fetching commits: %v\n", err)
	} else {
		for _, commit := range commits {
			if commit.Commit == nil || commit.Commit.Author == nil {
				continue
			}
			commitDate := commit.Commit.Author.Date.Time
			if commitDate.Before(start) || commitDate.After(end) {
				continue
			}
			repoName := extractRepoName(*commit.Repository.HTMLURL)
			task := report.Task{
				ID:          *commit.SHA,
				Title:       strings.Split(*commit.Commit.Message, "\n")[0], // First line
				Description: *commit.Commit.Message,
				Status:      "Committed",
				URL:         *commit.HTMLURL,
				CreatedAt:   commitDate,
				UpdatedAt:   commitDate,
				Source:      repoName,
				Type:        "Commit",
				Assignee:    g.Client.username,
			}
			allTasks = append(allTasks, task)
		}
	}

	// Fetch PRs
	prs, err := g.Client.FetchPRs(ctx, start, end)
	if err != nil {
		fmt.Printf("Error fetching PRs: %v\n", err)
	} else {
		for _, pr := range prs {
			if pr.CreatedAt == nil || pr.CreatedAt.Before(start) || pr.CreatedAt.After(end) {
				continue
			}
			repoName := extractRepoName(*pr.HTMLURL)
			var completedAt *time.Time
			if pr.ClosedAt != nil {
				t := pr.ClosedAt.Time
				completedAt = &t
			}
			status := *pr.State
			task := report.Task{
				ID:          fmt.Sprintf("%d", *pr.Number),
				Title:       *pr.Title,
				Description: "",
				Status:      status,
				URL:         *pr.HTMLURL,
				CreatedAt:   pr.CreatedAt.Time,
				UpdatedAt:   pr.UpdatedAt.Time,
				CompletedAt: completedAt,
				Source:      repoName,
				Type:        "Pull Request",
				Assignee:    g.Client.username,
			}
			if pr.Body != nil {
				task.Description = *pr.Body
			}
			allTasks = append(allTasks, task)
		}
	}

	// Fetch Issues
	issues, err := g.Client.FetchIssues(ctx, start, end)
	if err != nil {
		fmt.Printf("Error fetching issues: %v\n", err)
	} else {
		for _, issue := range issues {
			if issue.CreatedAt == nil || issue.CreatedAt.Before(start) || issue.CreatedAt.After(end) {
				continue
			}
			repoName := extractRepoName(*issue.HTMLURL)
			var completedAt *time.Time
			if issue.ClosedAt != nil {
				t := issue.ClosedAt.Time
				completedAt = &t
			}
			task := report.Task{
				ID:          fmt.Sprintf("%d", *issue.Number),
				Title:       *issue.Title,
				Description: "",
				Status:      *issue.State,
				URL:         *issue.HTMLURL,
				CreatedAt:   issue.CreatedAt.Time,
				UpdatedAt:   issue.UpdatedAt.Time,
				CompletedAt: completedAt,
				Source:      repoName,
				Type:        "Issue",
				Assignee:    g.Client.username,
			}
			if issue.Body != nil {
				task.Description = *issue.Body
			}
			allTasks = append(allTasks, task)
		}
	}

	return allTasks, nil
}

func extractRepoName(url string) string {
	// URL like https://github.com/owner/repo/pull/123 -> repo
	parts := strings.Split(url, "/")
	if len(parts) >= 5 {
		return parts[4]
	}
	return "unknown"
}
