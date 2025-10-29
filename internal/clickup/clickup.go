package clickup

import (
	"strconv"
	"time"

	"github.com/Afrawles/devreport/internal/report"
)

type ClickUpSource struct {
    Client *Client
}

func NewClickUpSource(apiKey, listID string, assigneeIDs []string) *ClickUpSource {
    return &ClickUpSource{
        Client: NewClient(apiKey, listID, assigneeIDs),
    }
}


var _ report.ActivitySource = (*ClickUpSource)(nil)

func (c *ClickUpSource) Name() string {
    return "ClickUp"
}

func (c *ClickUpSource) HealthCheck() error {
    return c.Client.HealthCheck()
}

func (c *ClickUpSource) FetchTasks(user string, start, end time.Time) ([]report.Task, error) {
    clickupTasks, err := c.Client.FetchTasks(start, end)
    if err != nil {
        return nil, err
    }

    tasks := []report.Task{}
    for _, t := range clickupTasks {

        createdMs, _ := strconv.ParseInt(t.DateCreated, 10, 64)
        updatedMs, _ := strconv.ParseInt(t.DateUpdated, 10, 64)

        createdAt := time.UnixMilli(createdMs)
        updatedAt := time.UnixMilli(updatedMs)

        var completedAt *time.Time
        if t.DateClosed != nil {
            closedMs, _ := strconv.ParseInt(*t.DateClosed, 10, 64)
            closed := time.UnixMilli(closedMs)
            completedAt = &closed
        }

        assignees := []string{}
        for _, a := range t.Assignees {
            assignees = append(assignees, a.Username)
        }

		// TODO: add other tasks

        task := report.Task{
            ID:          t.ID,
            Title:       t.Name,
            Description: t.Description,
            Status:      t.Status.Status,
            URL:         t.URL,
            CreatedAt:   createdAt,
            UpdatedAt:   updatedAt,
            CompletedAt: completedAt,
            Source:      c.Name(),
            Type:        "Task",
            Assignee:    assignees[0],
        }
        tasks = append(tasks, task)
    }

    return tasks, nil
}
