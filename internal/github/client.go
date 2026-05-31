package github

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

type Client struct {
	client                *github.Client
	orgs                  []string
	repos                 []string
	username              string
	includeReviewedPRs    bool
	includeAssignedIssues bool
	repoCache             map[string][]*github.Repository
}

func NewClient(token string, orgs []string, repos []string, username string, includeReviewedPRs, includeAssignedIssues bool) *Client {
	var tc oauth2.TokenSource
	if token != "" {
		tc = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	}
	httpClient := oauth2.NewClient(context.Background(), tc)
	client := github.NewClient(httpClient)

	return &Client{
		client:                client,
		orgs:                  orgs,
		repos:                 repos,
		username:              username,
		includeReviewedPRs:    includeReviewedPRs,
		includeAssignedIssues: includeAssignedIssues,
		repoCache:             make(map[string][]*github.Repository),
	}
}

func (c *Client) handleRateLimit(resp *github.Response, err error) error {
	if resp == nil {
		return err
	}

	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
				fmt.Printf("Rate limited. Waiting %d seconds...\n", seconds)
				time.Sleep(time.Duration(seconds) * time.Second)
				return nil
			}
		}

		remaining := resp.Header.Get("X-RateLimit-Remaining")
		reset := resp.Header.Get("X-RateLimit-Reset")
		if remaining == "0" && reset != "" {
			if resetTime, parseErr := strconv.ParseInt(reset, 10, 64); parseErr == nil {
				waitTime := time.Until(time.Unix(resetTime, 0))
				if waitTime > 0 {
					fmt.Printf("Rate limited. Waiting %v until reset...\n", waitTime)
					time.Sleep(waitTime)
					return nil
				}
			}
		}

		fmt.Println("Rate limited. Waiting 60 seconds...")
		time.Sleep(60 * time.Second)
		return nil
	}

	return err
}

func (c *Client) handleRateLimitWithRetry(attempt int) {
	delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
	time.Sleep(delay)
}

func (c *Client) HealthCheck() error {
	_, _, err := c.client.Zen(context.Background())
	return err
}

type PRWithCommits struct {
	PR      *github.PullRequest
	Commits []*github.RepositoryCommit
}

// FetchPRsWithCommits fetches PRs authored by the user and attaches their commits.
// This is the primary fetch — commits are derived from PRs, not searched separately.
func (c *Client) FetchPRsWithCommits(ctx context.Context, start, end time.Time) ([]*PRWithCommits, error) {
	var results []*PRWithCommits
	seen := make(map[string]bool)

	for _, org := range c.orgs {
		repos, err := c.getOrgRepos(ctx, org)
		if err != nil {
			fmt.Printf("Warning: could not list repos for org %s: %v\n", org, err)
			continue
		}

		for _, repo := range repos {
			prs, err := c.fetchPRsInRepo(ctx, org, *repo.Name, start, end)
			if err != nil {
				fmt.Printf("Warning: could not fetch PRs for %s/%s: %v\n", org, *repo.Name, err)
				continue
			}

			for _, pr := range prs {
				key := fmt.Sprintf("%s/%s#%d", org, *repo.Name, *pr.Number)
				if seen[key] {
					continue
				}
				seen[key] = true

				commits, err := c.fetchPRCommits(ctx, org, *repo.Name, *pr.Number)
				if err != nil {
					fmt.Printf("Warning: could not fetch commits for PR %s: %v\n", key, err)
					commits = nil
				}

				results = append(results, &PRWithCommits{
					PR:      pr,
					Commits: commits,
				})
			}
		}
	}

	return results, nil
}

func (c *Client) fetchPRsInRepo(ctx context.Context, org, repo string, start, end time.Time) ([]*github.PullRequest, error) {
	var prs []*github.PullRequest

	opts := &github.PullRequestListOptions{
		State:       "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		result, resp, err := c.client.PullRequests.List(ctx, org, repo, opts)
		if err != nil {
			if rateErr := c.handleRateLimit(resp, err); rateErr != nil {
				return nil, rateErr
			}
			return nil, err
		}

		for _, pr := range result {
			if pr.CreatedAt == nil {
				continue
			}
			if pr.CreatedAt.After(end) {
				continue
			}
			if pr.CreatedAt.Before(start) {
				return prs, nil
			}
			if pr.User == nil || pr.User.Login == nil {
				continue
			}
			if !strings.EqualFold(*pr.User.Login, c.username) {
				continue
			}
			prs = append(prs, pr)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
		time.Sleep(100 * time.Millisecond)
	}

	return prs, nil
}

func (c *Client) fetchPRCommits(ctx context.Context, org, repo string, prNumber int) ([]*github.RepositoryCommit, error) {
	var commits []*github.RepositoryCommit

	opts := &github.ListOptions{PerPage: 100}
	for {
		result, resp, err := c.client.PullRequests.ListCommits(ctx, org, repo, prNumber, opts)
		if err != nil {
			if rateErr := c.handleRateLimit(resp, err); rateErr != nil {
				return nil, rateErr
			}
			return nil, err
		}
		commits = append(commits, result...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
		time.Sleep(100 * time.Millisecond)
	}

	return commits, nil
}

// FetchIssues fetches issues created by the user in the date range.
func (c *Client) FetchIssues(ctx context.Context, start, end time.Time) ([]*github.Issue, error) {
	var allIssues []*github.Issue
	issueMap := make(map[string]bool)

	for _, org := range c.orgs {
		repos, err := c.getOrgRepos(ctx, org)
		if err != nil {
			fmt.Printf("Warning: could not list repos for org %s: %v\n", org, err)
			continue
		}

		for _, repo := range repos {
			opts := &github.IssueListByRepoOptions{
				State:       "all",
				Creator:     c.username,
				Since:       start,
				ListOptions: github.ListOptions{PerPage: 100},
			}

			for {
				issues, resp, err := c.client.Issues.ListByRepo(ctx, org, *repo.Name, opts)
				if err != nil {
					if rateErr := c.handleRateLimit(resp, err); rateErr != nil {
						return nil, rateErr
					}
					break
				}

				for _, issue := range issues {
					if issue.IsPullRequest() {
						continue
					}
					if issue.CreatedAt == nil || issue.CreatedAt.Before(start) || issue.CreatedAt.After(end) {
						continue
					}
					key := fmt.Sprintf("%s/%s#%d", org, *repo.Name, *issue.Number)
					if issueMap[key] {
						continue
					}
					issueMap[key] = true
					allIssues = append(allIssues, issue)
				}

				if resp.NextPage == 0 {
					break
				}
				opts.Page = resp.NextPage
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	if c.includeAssignedIssues {
		assigned, err := c.fetchAssignedIssues(ctx, start, end)
		if err != nil {
			return nil, err
		}
		for _, issue := range assigned {
			if issue.HTMLURL == nil {
				continue
			}
			key := *issue.HTMLURL
			if !issueMap[key] {
				issueMap[key] = true
				allIssues = append(allIssues, issue)
			}
		}
	}

	return allIssues, nil
}

func (c *Client) fetchAssignedIssues(ctx context.Context, start, end time.Time) ([]*github.Issue, error) {
	var allIssues []*github.Issue
	seen := make(map[string]bool)

	for _, org := range c.orgs {
		query := fmt.Sprintf("assignee:%s type:issue created:%s..%s org:%s",
			c.username, start.Format("2006-01-02"), end.Format("2006-01-02"), org)

		opts := &github.SearchOptions{
			ListOptions: github.ListOptions{PerPage: 100},
		}

		for {
			result, resp, err := c.client.Search.Issues(ctx, query, opts)
			if err != nil {
				if rateErr := c.handleRateLimit(resp, err); rateErr != nil {
					return nil, rateErr
				}
				break
			}

			for _, issue := range result.Issues {
				if issue.HTMLURL == nil || seen[*issue.HTMLURL] {
					continue
				}
				seen[*issue.HTMLURL] = true
				allIssues = append(allIssues, issue)
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
			time.Sleep(100 * time.Millisecond)
		}
	}

	return allIssues, nil
}

func (c *Client) getOrgRepos(ctx context.Context, org string) ([]*github.Repository, error) {
	if repos, ok := c.repoCache[org]; ok {
		return repos, nil
	}

	if len(c.repos) > 0 {
		var filtered []*github.Repository
		for _, repoName := range c.repos {
			repoName = strings.TrimSpace(repoName)
			if repoName == "" {
				continue
			}
			owner := org
			name := repoName
			if strings.Contains(repoName, "/") {
				parts := strings.SplitN(repoName, "/", 2)
				owner, name = parts[0], parts[1]
			}
			repo, _, err := c.client.Repositories.Get(ctx, owner, name)
			if err != nil {
				fmt.Printf("Warning: could not fetch repo %s/%s: %v\n", owner, name, err)
				continue
			}
			filtered = append(filtered, repo)
		}
		c.repoCache[org] = filtered
		return filtered, nil
	}

	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var repos []*github.Repository
	for {
		result, resp, err := c.client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			if rateErr := c.handleRateLimit(resp, err); rateErr != nil {
				return nil, rateErr
			}
			return nil, err
		}
		repos = append(repos, result...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
		time.Sleep(100 * time.Millisecond)
	}

	c.repoCache[org] = repos
	return repos, nil
}
