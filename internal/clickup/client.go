package clickup

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

)

const baseURL = "https://api.clickup.com/api/v2"

type Client struct {
    apiKey      string
    assigneeIDs []string
    httpClient  *http.Client
	listID string
}

func NewClient(apiKey, listID string, assigneeIDs []string) *Client {
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
}

type ClickUpStatus struct {
    Status string `json:"status"`
}

type Assignee struct {
    ID       int    `json:"id"`
    Username string `json:"username"`
}

type TasksResponse struct {
    Tasks []ClickUpTask `json:"tasks"`
}

// FetchTasks fetches tasks for assigned users within date range
func (c *Client) FetchTasks(start, end time.Time) ([]ClickUpTask, error) {
	var assigneeParams string
	for _, id := range c.assigneeIDs {
		assigneeParams += fmt.Sprintf("&assignees=%s", id)
	}

	startMs := start.UnixMilli()
	endMs := end.UnixMilli()

	url := fmt.Sprintf("%s/list/%s/task?order_by=created&subtasks=true&include_closed=true&include_timl=true%s&date_created_gt=%d&date_created_lt=%d",
		baseURL, c.listID, assigneeParams, startMs, endMs)

	fmt.Println(url)

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
