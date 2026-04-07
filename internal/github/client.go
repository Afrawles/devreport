package github

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

type Client struct {
	client                *github.Client
	orgs                  []string
	username              string
	includeReviewedPRs    bool
	includeAssignedIssues bool
}

func NewClient(token string, orgs []string, username string, includeReviewedPRs, includeAssignedIssues bool) *Client {
	var tc oauth2.TokenSource
	if token != "" {
		tc = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	}
	httpClient := oauth2.NewClient(context.Background(), tc)
	client := github.NewClient(httpClient)

	return &Client{
		client:                client,
		orgs:                  orgs,
		username:              username,
		includeReviewedPRs:    includeReviewedPRs,
		includeAssignedIssues: includeAssignedIssues,
	}
}

// makeRequestWithRetry executes a GitHub API request with rate limit handling and retry logic
func (c *Client) makeRequestWithRetry(ctx context.Context, fn func() (*http.Response, error)) (*http.Response, error) {
	const maxRetries = 5
	baseDelay := time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := fn()
		if err != nil {
			return resp, err
		}

		// Check for rate limit errors
		if resp.StatusCode == 403 || resp.StatusCode == 429 {
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					time.Sleep(time.Duration(seconds) * time.Second)
					continue
				}
			}

			// Check rate limit headers
			remaining := resp.Header.Get("X-RateLimit-Remaining")
			reset := resp.Header.Get("X-RateLimit-Reset")
			if remaining == "0" && reset != "" {
				if resetTime, err := strconv.ParseInt(reset, 10, 64); err == nil {
					waitTime := time.Until(time.Unix(resetTime, 0))
					if waitTime > 0 {
						time.Sleep(waitTime)
						continue
					}
				}
			}

			// Exponential backoff for secondary rate limits
			delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
			time.Sleep(delay)
			continue
		}

		// Success or non-rate-limit error
		return resp, err
	}

	return nil, fmt.Errorf("max retries exceeded")
}

// handleRateLimit checks for rate limit errors and sleeps if necessary
func (c *Client) handleRateLimit(resp *github.Response, err error) error {
	if resp == nil {
		return err
	}

	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				fmt.Printf("Rate limited. Waiting %d seconds...\n", seconds)
				time.Sleep(time.Duration(seconds) * time.Second)
				return nil // Retry
			}
		}

		remaining := resp.Header.Get("X-RateLimit-Remaining")
		reset := resp.Header.Get("X-RateLimit-Reset")
		if remaining == "0" && reset != "" {
			if resetTime, err := strconv.ParseInt(reset, 10, 64); err == nil {
				waitTime := time.Until(time.Unix(resetTime, 0))
				if waitTime > 0 {
					fmt.Printf("Rate limited. Waiting %v until reset...\n", waitTime)
					time.Sleep(waitTime)
					return nil // Retry
				}
			}
		}

		// If we can't determine wait time, wait 60 seconds
		fmt.Println("Rate limited. Waiting 60 seconds...")
		time.Sleep(60 * time.Second)
		return nil
	}

	return err
}

func (c *Client) HealthCheck() error {
	_, _, err := c.client.Zen(context.Background())
	return err
}

func (c *Client) FetchCommits(ctx context.Context, start, end time.Time) ([]*github.CommitResult, error) {
	var allCommits []*github.CommitResult

	query := fmt.Sprintf("author:%s committer-date:%s..%s", c.username, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if len(c.orgs) > 0 {
		orgQuery := strings.Join(c.orgs, " org:")
		query += " org:" + orgQuery
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		commits, resp, err := c.client.Search.Commits(ctx, query, opts)
		if err != nil {
			// Check if it's a rate limit error and retry
			if rateErr := c.handleRateLimit(resp, err); rateErr != nil {
				return nil, rateErr
			}
			return nil, err
		}
		allCommits = append(allCommits, commits.Commits...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage

		// Small delay between requests to avoid secondary rate limits
		time.Sleep(100 * time.Millisecond)
	}

	return allCommits, nil
}

func (c *Client) FetchPRs(ctx context.Context, start, end time.Time) ([]*github.Issue, error) {
	var allPRs []*github.Issue

	query := fmt.Sprintf("author:%s type:pr created:%s..%s", c.username, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if len(c.orgs) > 0 {
		orgQuery := strings.Join(c.orgs, " org:")
		query += " org:" + orgQuery
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		result, resp, err := c.client.Search.Issues(ctx, query, opts)
		if err != nil {
			if rateErr := c.handleRateLimit(resp, err); rateErr != nil {
				return nil, rateErr
			}
			return nil, err
		}
		allPRs = append(allPRs, result.Issues...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	// If include reviewed PRs
	if c.includeReviewedPRs {
		reviewedPRs, err := c.fetchReviewedPRs(ctx, start, end)
		if err != nil {
			return nil, err
		}
		// Merge, avoiding duplicates
		prMap := make(map[int64]*github.Issue)
		for _, pr := range allPRs {
			prMap[int64(*pr.Number)] = pr
		}
		for _, pr := range reviewedPRs {
			if _, exists := prMap[int64(*pr.Number)]; !exists {
				allPRs = append(allPRs, pr)
			}
		}
	}

	return allPRs, nil
}

func (c *Client) fetchReviewedPRs(ctx context.Context, start, end time.Time) ([]*github.Issue, error) {
	var allPRs []*github.Issue

	for _, org := range c.orgs {
		repos, err := c.getOrgRepos(ctx, org)
		if err != nil {
			continue
		}
		for _, repo := range repos {
			repoName := *repo.Name
			prs, err := c.getReviewedPRsInRepo(ctx, org, repoName, start, end)
			if err != nil {
				continue
			}
			allPRs = append(allPRs, prs...)
		}
	}

	return allPRs, nil
}

func (c *Client) getOrgRepos(ctx context.Context, org string) ([]*github.Repository, error) {
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

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}
	return repos, nil
}

func (c *Client) getReviewedPRsInRepo(ctx context.Context, owner, repo string, start, end time.Time) ([]*github.Issue, error) {
	var prs []*github.Issue

	opts := &github.PullRequestListOptions{
		State:       "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		result, resp, err := c.client.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			if rateErr := c.handleRateLimit(resp, err); rateErr != nil {
				return nil, rateErr
			}
			return nil, err
		}

		for _, pr := range result {
			if pr.CreatedAt.After(end) || pr.CreatedAt.Before(start) {
				continue
			}
			// Check if user reviewed
			reviews, _, err := c.client.PullRequests.ListReviews(ctx, owner, repo, *pr.Number, nil)
			if err != nil {
				// Skip if can't get reviews, but don't fail
				continue
			}
			for _, review := range reviews {
				if review.User != nil && *review.User.Login == c.username {
					prs = append(prs, &github.Issue{
						Number:    pr.Number,
						Title:     pr.Title,
						Body:      pr.Body,
						State:     pr.State,
						HTMLURL:   pr.HTMLURL,
						User:      pr.User,
						CreatedAt: pr.CreatedAt,
						UpdatedAt: pr.UpdatedAt,
						ClosedAt:  pr.ClosedAt,
					})
					break
				}
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	return prs, nil
}

func (c *Client) FetchIssues(ctx context.Context, start, end time.Time) ([]*github.Issue, error) {
	var allIssues []*github.Issue

	query := fmt.Sprintf("author:%s type:issue created:%s..%s", c.username, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if len(c.orgs) > 0 {
		orgQuery := strings.Join(c.orgs, " org:")
		query += " org:" + orgQuery
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		result, resp, err := c.client.Search.Issues(ctx, query, opts)
		if err != nil {
			if rateErr := c.handleRateLimit(resp, err); rateErr != nil {
				return nil, rateErr
			}
			return nil, err
		}
		allIssues = append(allIssues, result.Issues...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	// If include assigned issues
	if c.includeAssignedIssues {
		assignedIssues, err := c.fetchAssignedIssues(ctx, start, end)
		if err != nil {
			return nil, err
		}
		// Merge, avoiding duplicates
		issueMap := make(map[int64]*github.Issue)
		for _, issue := range allIssues {
			issueMap[int64(*issue.Number)] = issue
		}
		for _, issue := range assignedIssues {
			if _, exists := issueMap[int64(*issue.Number)]; !exists {
				allIssues = append(allIssues, issue)
			}
		}
	}

	return allIssues, nil
}

func (c *Client) fetchAssignedIssues(ctx context.Context, start, end time.Time) ([]*github.Issue, error) {
	var allIssues []*github.Issue

	query := fmt.Sprintf("assignee:%s type:issue created:%s..%s", c.username, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if len(c.orgs) > 0 {
		orgQuery := strings.Join(c.orgs, " org:")
		query += " org:" + orgQuery
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		result, resp, err := c.client.Search.Issues(ctx, query, opts)
		if err != nil {
			if rateErr := c.handleRateLimit(resp, err); rateErr != nil {
				return nil, rateErr
			}
			return nil, err
		}
		allIssues = append(allIssues, result.Issues...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	return allIssues, nil
}
