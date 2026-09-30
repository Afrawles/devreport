package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Afrawles/devreport/internal/llm"
	"github.com/Afrawles/devreport/internal/report"
	gogithub "github.com/google/go-github/v60/github"
)

const defaultOllamaModel = "gemma4:e4b"

type GitHubSource struct {
	Client *Client
	LLM    llm.Provider
}

func NewGitHubSource(token string, orgs []string, username string, repos []string, includeReviewedPRs, includeAssignedIssues bool, llmCfg llm.Config) *GitHubSource {
	llmCfg.OllamaModel = defaultOllamaModel
	return &GitHubSource{
		Client: NewClient(token, orgs, repos, username, includeReviewedPRs, includeAssignedIssues),
		LLM:    llm.New(llmCfg),
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

			achievement := g.buildAchievement(ctx, *pr.HTMLURL, *pr.Number, title, body, entry.Commits)

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

			achievement := rephraseCommit(g.LLM, title)

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

// buildAchievement produces the report achievement text for a PR. When the
// available title/body/commit text is too weak to describe the work, it
// falls back to the PR diff instead of rephrasing near-empty input.
func (g *GitHubSource) buildAchievement(ctx context.Context, htmlURL string, prNumber int, title, body string, commits []*gogithub.RepositoryCommit) string {
	if isWeakInput(title, body, commits) {
		owner, repo := extractOwnerRepo(htmlURL)
		if owner != "" && repo != "" {
			diff, err := g.Client.fetchPRDiff(ctx, owner, repo, prNumber)
			if err == nil && strings.TrimSpace(diff) != "" {
				return describeDiff(g.LLM, title, diff)
			}
			fmt.Printf("Diff fetch failed for PR #%d, falling back to text: %v\n", prNumber, err)
		}
	}

	achievementInput := buildAchievementInput(title, body, commits)
	return rephrasePR(g.LLM, title, achievementInput)
}

// isWeakInput reports whether the PR body, commit messages, and title all
// carry too little signal to describe the change without looking at the diff.
func isWeakInput(title, body string, commits []*gogithub.RepositoryCommit) bool {
	if len(strings.TrimSpace(body)) >= 20 {
		return false
	}
	if len(strings.TrimSpace(title)) >= 25 {
		return false
	}
	for _, c := range commits {
		if c.Commit == nil || c.Commit.Message == nil {
			continue
		}
		msg := cleanActivityText(*c.Commit.Message)
		if !shouldSkipCommitMessage(msg) && len(strings.TrimSpace(msg)) >= 15 {
			return false
		}
	}
	return true
}

// buildAchievementInput returns the best available text to feed into the LLM.
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

// extractOwnerRepo parses owner/repo out of a PR/issue HTML URL, e.g.
// https://github.com/{owner}/{repo}/pull/{n}.
func extractOwnerRepo(url string) (owner, repo string) {
	parts := strings.Split(url, "/")
	if len(parts) >= 5 {
		return parts[3], parts[4]
	}
	return "", ""
}

func rephrasePR(p llm.Provider, title, body string) string {
	input := title
	if strings.TrimSpace(body) != "" {
		input = title + "\n\n" + body
	}

	if strings.TrimSpace(input) == "" {
		return input
	}

	prompt := "Rephrase the following pull request title and description as a concise, professional achievement bullet point.\n\n" +
		"STRICT RULES:\n" +
		"1. Use strong action verbs and focus on the accomplishment\n" +
		"2. PRESERVE all numerical values, version numbers, and identifiers EXACTLY as written\n" +
		"3. Only fix spelling errors and grammar mistakes\n" +
		"4. Do NOT change the core meaning or technical details\n" +
		"5. Keep it concise — one to two sentences max\n" +
		"6. Return only the rephrased text without bullet point symbols (•, -, *)\n\n" +
		"Pull request:\n" +
		input

	rephrased, err := p.Complete(prompt)
	if err != nil {
		fmt.Printf("%s unavailable for PR rephrase: %v\n", p.Name(), err)
		return title
	}

	return rephrased
}

func rephraseCommit(p llm.Provider, message string) string {
	if strings.TrimSpace(message) == "" {
		return message
	}

	prompt := "Rephrase the following git commit message as a concise, professional achievement bullet point.\n\n" +
		"STRICT RULES:\n" +
		"1. Use strong action verbs and focus on the accomplishment\n" +
		"2. PRESERVE all numerical values, version numbers, and identifiers EXACTLY as written\n" +
		"3. Only fix spelling errors and grammar mistakes\n" +
		"4. Do NOT change the core meaning or technical details\n" +
		"5. Keep it concise — one sentence max\n" +
		"6. Return only the rephrased text without bullet point symbols (•, -, *)\n\n" +
		"Commit message:\n" +
		message

	rephrased, err := p.Complete(prompt)
	if err != nil {
		fmt.Printf("%s unavailable for commit rephrase: %v\n", p.Name(), err)
		return message
	}

	return rephrased
}

func describeDiff(p llm.Provider, title, diff string) string {
	prompt := "The following pull request has a missing or unhelpful description. " +
		"Based on the code diff below, write a concise, professional one-to-two " +
		"sentence achievement bullet point describing what was actually changed.\n\n" +
		"STRICT RULES:\n" +
		"1. Use strong action verbs and focus on the accomplishment\n" +
		"2. Base the description only on what the diff shows — do not invent context\n" +
		"3. Keep it concise — one to two sentences max\n" +
		"4. Return only the rephrased text without bullet point symbols (•, -, *)\n\n" +
		"PR title: " + title + "\n\nDiff:\n" + diff

	result, err := p.Complete(prompt)
	if err != nil || strings.TrimSpace(result) == "" {
		fmt.Printf("%s unavailable for diff description: %v\n", p.Name(), err)
		return title
	}
	return strings.TrimSpace(result)
}
