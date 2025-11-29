package clickup

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const baseURL = "https://api.clickup.com/api/v2"

type Client struct {
    apiKey      string
    assigneeIDs []string
    httpClient  *http.Client
	listID []string
}

func NewClient(apiKey string, listID, assigneeIDs []string) *Client {
    return &Client{
        apiKey:      apiKey,
        assigneeIDs: assigneeIDs,
        httpClient:  &http.Client{Timeout: 30 * time.Second},
		listID: listID,
    }
}

type ClickUpTask struct {
    ID          string      `json:"id"`
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Status      ClickUpStatus `json:"status"`
    URL         string      `json:"url"`
    DateCreated string      `json:"date_created"`
    DateUpdated string      `json:"date_updated"`
    DateClosed  *string     `json:"date_closed"`
    Assignees   []Assignee  `json:"assignees"`
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
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Folder FolderInfo  `json:"folder"`
	Space  SpaceInfo   `json:"space"`
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

// FetchTasksForList fetches tasks for assigned users, in single List within date range
func (c *Client) FetchTasksForList(listID string, start, end time.Time) ([]ClickUpTask, error) {
	var assigneeParams string
	for _, id := range c.assigneeIDs {
		assigneeParams += fmt.Sprintf("&assignees=%s", id)
	}

	startMs := start.UnixMilli()
	endMs := end.UnixMilli()

	url := fmt.Sprintf("%s/list/%s/task?order_by=created&include_closed=true&include_timl=true%s&date_created_gt=%d&date_created_lt=%d",
		baseURL, listID, assigneeParams, startMs, endMs)

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Authorization", c.apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
    }

    var result TasksResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return result.Tasks, nil
}

func (c *Client) FetchTasks(listIDs []string, start, end time.Time, maxWorkers int) ([]ClickUpTask, error) {
	type result struct {
		tasks []ClickUpTask
		err   error
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
				if err != nil {
					resultCh <- result{err: fmt.Errorf("list %s: %w", listID, err)}
				} else {
					resultCh <- result{tasks: tasks}
				}
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
