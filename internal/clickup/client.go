package clickup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	baseURL = "https://api.clickup.com/api/v2"

	//TODO: config
	defaultMaxWorkers = 5
	requestsPerMinute = 100
	burst             = 10
	retryMax          = 5
)

type Client struct {
	apiKey      string
	assigneeIDs []string
	httpClient  *http.Client
	limiter     *rate.Limiter
	listID      []string
	listNames   map[string]string
}

func NewClient(apiKey string, listID, assigneeIDs []string) *Client {
	return &Client{
		apiKey:      apiKey,
		assigneeIDs: assigneeIDs,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		listID:      listID,
		listNames:   make(map[string]string),
		limiter:     rate.NewLimiter(rate.Every(time.Minute/requestsPerMinute), burst),
	}
}

func (c *Client) wait() error {
	return c.limiter.Wait(context.Background())
}

func (c *Client) GetListName(listID string) string {
	if name, ok := c.listNames[listID]; ok {
		return name
	}
	return ""
}

func (c *Client) SetListNames(names map[string]string) {
	c.listNames = names
}

type ClickUpTask struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Status      ClickUpStatus `json:"status"`
	URL         string        `json:"url"`
	DateCreated string        `json:"date_created"`
	DateUpdated string        `json:"date_updated"`
	DateClosed  *string       `json:"date_closed"`
	Assignees   []Assignee    `json:"assignees"`
	List        ListInfo      `json:"list"`
}

type ClickUpStatus struct {
	Status string `json:"status"`
}

type Assignee struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type ListInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ListDetails struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Folder FolderInfo `json:"folder"`
	Space  SpaceInfo  `json:"space"`
}

type FolderInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SpaceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TasksResponse struct {
	Tasks []ClickUpTask `json:"tasks"`
}

// FetchListDetails fetches details for a specific list
func (c *Client) FetchListDetails(listID string) (*ListDetails, error) {
	url := fmt.Sprintf("%s/list/%s", baseURL, listID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result ListDetails
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// FetchTasksForList fetches ALL tasks in a list
func (c *Client) FetchTasksForList(listID string, start, end time.Time) ([]ClickUpTask, error) {
	var allTasks []ClickUpTask
	page := 0

	for {
		if err := c.wait(); err != nil {
			return nil, err
		}

		base := fmt.Sprintf("%s/list/%s/task", baseURL, listID)
		q := url.Values{}
		q.Add("subtasks", "true")
		q.Add("include_timl", "true")
		q.Add("order_by", "created")
		q.Add("include_closed", "true")
		q.Add("page", fmt.Sprintf("%d", page))

		if len(c.assigneeIDs) > 0 {
			for _, id := range c.assigneeIDs {
				q.Add("assignees[]", id)
			}
		}

		if !start.IsZero() {
			q.Add("date_created_gt", fmt.Sprintf("%d", start.UnixMilli()))
		}
		if !end.IsZero() {
			q.Add("date_created_lt", fmt.Sprintf("%d", end.UnixMilli()))
		}

		req, err := http.NewRequest("GET", base+"?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", c.apiKey)

		resp, err := c.doWithRetry(req)
		if err != nil {
			return nil, fmt.Errorf("request failed after retries: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
		}

		var result TasksResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		if len(result.Tasks) == 0 {
			break
		}

		allTasks = append(allTasks, result.Tasks...)
		fmt.Printf("  Fetched page %d → %d tasks (total: %d)\n", page, len(result.Tasks), len(allTasks))

		page++
	}

	return allTasks, nil
}

func (c *Client) FetchTasks(listIDs []string, start, end time.Time, maxWorkers int) ([]ClickUpTask, error) {
	type result struct {
		tasks []ClickUpTask
		err   error
	}

	if maxWorkers <= 0 {
		maxWorkers = 5
	}

	resultCh := make(chan result, len(listIDs))
	taskCh := make(chan string, len(listIDs))

	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for listID := range taskCh {
				tasks, err := c.FetchTasksForList(listID, start, end)
				resultCh <- result{tasks: tasks, err: err}
			}
		}()
	}

	for _, id := range listIDs {
		taskCh <- id
	}
	close(taskCh)

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var allTasks []ClickUpTask
	var allErrs []error

	for res := range resultCh {
		if res.err != nil {
			allErrs = append(allErrs, res.err)
		} else {
			allTasks = append(allTasks, res.tasks...)
		}
	}

	if len(allErrs) > 0 {
		return allTasks, fmt.Errorf("some requests failed: %v", allErrs)
	}

	return allTasks, nil
}

func (c *Client) HealthCheck() error {
	req, err := http.NewRequest("GET", baseURL+"/user", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// doWithRetry performs request with exponential backoff on 429/5xx
func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= retryMax; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			time.Sleep(backoff)
		}

		if err := c.wait(); err != nil {
			return nil, err
		}

		resp, err = c.httpClient.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("exhausted retries: last error: %v", err)
}

// GetListIDsAndNamesFromFolder fetches list IDs and names from a folder
func (c *Client) GetListIDsAndNamesFromFolder(folderID string) ([]string, map[string]string, error) {
	var ids []string
	names := make(map[string]string)

	url := fmt.Sprintf("%s/folder/%s/list", baseURL, folderID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", c.apiKey)

	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Lists []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"lists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, err
	}

	if len(result.Lists) == 0 {
		return nil, nil, nil
	}

	for _, l := range result.Lists {
		ids = append(ids, l.ID)
		names[l.ID] = l.Name
	}

	return ids, names, nil
}

//NOTE: Deprecated: Use GetListIDsAndNamesFromFolder instead
func (c *Client) GetListIDsFromFolder(folderID string) ([]string, error) {
	ids, _, err := c.GetListIDsAndNamesFromFolder(folderID)
	return ids, err
}
