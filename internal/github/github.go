package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Afrawles/devreport/internal/report"
	gogithub "github.com/google/go-github/v60/github"
)

type GitHubSource struct {
	Client *Client
}

func NewGitHubSource(token string, orgs []string, username string, repos []string, includeReviewedPRs, includeAssignedIssues bool) *GitHubSource {
	return &GitHubSource{
		Client: NewClient(token, orgs, repos, username, includeReviewedPRs, includeAssignedIssues),
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

	prsWithCommits, err := g.Client.FetchPRsWithCommits(ctx, start, end)
	if err != nil {
		fmt.Printf("Error fetching PRs: %v\n", err)
	} else {
		for _, entry := range prsWithCommits {
			pr := entry.PR
			if pr.CreatedAt == nil || pr.HTMLURL == nil || pr.Number == nil || pr.Title == nil || pr.State == nil || pr.UpdatedAt == nil {
				continue
			}

			repoName := extractRepoName(*pr.HTMLURL)
			var completedAt *time.Time
			if pr.ClosedAt != nil {
				t := pr.ClosedAt.Time
				completedAt = &t
			}

			title := *pr.Title
			body := ""
			if pr.Body != nil {
				body = cleanActivityText(*pr.Body)
			}

			achievementInput := buildAchievementInput(title, body, entry.Commits)
			achievement := rephrasePR(title, achievementInput)

			task := report.Task{
				ID:           fmt.Sprintf("%d", *pr.Number),
				Title:        title,
				Description:  body,
				Achievements: achievement,
				Status:       *pr.State,
				URL:          *pr.HTMLURL,
				CreatedAt:    pr.CreatedAt.Time,
				UpdatedAt:    pr.UpdatedAt.Time,
				CompletedAt:  completedAt,
				Source:       repoName,
				Type:         "Pull Request",
				Assignee:     g.Client.username,
			}
			allTasks = append(allTasks, task)
		}
	}

	issues, err := g.Client.FetchIssues(ctx, start, end)
	if err != nil {
		fmt.Printf("Error fetching issues: %v\n", err)
	} else {
		for _, issue := range issues {
			if issue.CreatedAt == nil || issue.HTMLURL == nil || issue.Number == nil || issue.Title == nil || issue.State == nil || issue.UpdatedAt == nil {
				continue
			}

			repoName := extractRepoName(*issue.HTMLURL)
			var completedAt *time.Time
			if issue.ClosedAt != nil {
				t := issue.ClosedAt.Time
				completedAt = &t
			}

			title := *issue.Title
			body := ""
			if issue.Body != nil {
				body = cleanActivityText(*issue.Body)
			}

			achievement := rephraseCommit(title)

			task := report.Task{
				ID:           fmt.Sprintf("%d", *issue.Number),
				Title:        title,
				Description:  body,
				Achievements: achievement,
				Status:       *issue.State,
				URL:          *issue.HTMLURL,
				CreatedAt:    issue.CreatedAt.Time,
				UpdatedAt:    issue.UpdatedAt.Time,
				CompletedAt:  completedAt,
				Source:       repoName,
				Type:         "Issue",
				Assignee:     g.Client.username,
			}
			allTasks = append(allTasks, task)
		}
	}

	return allTasks, nil
}

// buildAchievementInput returns the best available text to feed into Ollama.
// Priority: PR body > commit messages > PR title only.
func buildAchievementInput(title, body string, commits []*gogithub.RepositoryCommit) string {
	if strings.TrimSpace(body) != "" {
		return body
	}

	if len(commits) > 0 {
		var lines []string
		for _, c := range commits {
			if c.Commit == nil || c.Commit.Message == nil {
				continue
			}
			msg := cleanActivityText(*c.Commit.Message)
			if shouldSkipCommitMessage(msg) {
				continue
			}
			lines = append(lines, msg)
		}
		if len(lines) > 0 {
			return strings.Join(lines, "\n\n")
		}
	}

	return title
}

func shouldSkipCommitMessage(message string) bool {
	first := firstLine(message)
	if first == "" {
		return true
	}
	lower := strings.ToLower(first)
	for _, prefix := range []string{
		"merge pull request #",
		"merge branch ",
		"merge remote-tracking branch ",
		"merge tag ",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(value, "\n")[0])
}

func cleanActivityText(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractRepoName(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) >= 5 {
		return parts[4]
	}
	return "unknown"
}
