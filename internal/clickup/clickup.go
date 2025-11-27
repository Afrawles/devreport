package clickup

import (
	"strconv"
	"strings"
	"time"

	"github.com/Afrawles/devreport/internal/report"
)

type ClickUpSource struct {
	Client   *Client
	Category string
}

func NewClickUpSource(apiKey string, listID, assigneeIDs []string, category string) *ClickUpSource {
	return &ClickUpSource{
		Client:   NewClient(apiKey, listID, assigneeIDs),
		Category: category,
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
	clickupTasks, err := c.Client.FetchTasks(c.Client.listID, start, end, len(c.Client.listID))
	if err != nil {
		return nil, err
	}

	var allTasks []report.Task

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

		var assigneeNames []string
		for _, a := range t.Assignees {
			assigneeNames = append(assigneeNames, a.Username)
		}
		assignee := strings.Join(assigneeNames, ", ")
		if assignee == "" {
			assignee = ""
		}

		projectName := c.Client.GetListName(t.List.ID)
		if projectName == "" {
			projectName = t.List.Name
		}

		task := report.Task{
			ID:              t.ID,
			Title:           t.Name,
			Description:     t.Description,
			Achievements:    t.Description,
			Status:          t.Status.Status,
			URL:             t.URL,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
			CompletedAt:     completedAt,
			Source:          projectName,
			Type:            "Task",
			Assignee:        assignee,
			Challenges:      "",
			SupportRequired: "",
			SupportFrom:     "",
			FollowUp:        "",
		}

		allTasks = append(allTasks, task)
	}

	return allTasks, nil
}
