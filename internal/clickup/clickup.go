package clickup

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Afrawles/devreport/internal/report"
)

type ClickUpSource struct {
    Client *Client
	Category string
}

func NewClickUpSource(apiKey string, listID, assigneeIDs []string, category string) *ClickUpSource {
    return &ClickUpSource{
        Client: NewClient(apiKey, listID, assigneeIDs),
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

	tasksByList := make(map[string][]ClickUpTask)
	for _, t := range clickupTasks {
		tasksByList[t.List.ID] = append(tasksByList[t.List.ID], t)
	}

	var groupedTasks []report.Task
	for listID, tasksInList := range tasksByList {
		if len(tasksInList) == 0 {
			continue
		}

		listDetails, err := c.Client.FetchListDetails(listID)
		if err != nil {
			fmt.Printf("Warning: could not fetch details for list %s: %v\n", listID, err)
			continue
		}

		keyActivity := listDetails.Name
		if c.Category != "" {
			keyActivity = fmt.Sprintf("%s %s", listDetails.Name, c.Category)
		}

		var achievements []string
		var latestUpdate time.Time
		var completionDate *time.Time
		allCompleted := true

		for _, t := range tasksInList {
			achievements = append(achievements, fmt.Sprintf("• %s", t.Name))
			
			updatedMs, _ := strconv.ParseInt(t.DateUpdated, 10, 64)
			updatedAt := time.UnixMilli(updatedMs)
			if updatedAt.After(latestUpdate) {
				latestUpdate = updatedAt
			}

			if t.DateClosed != nil {
				closedMs, _ := strconv.ParseInt(*t.DateClosed, 10, 64)
				closed := time.UnixMilli(closedMs)
				if completionDate == nil || closed.After(*completionDate) {
					completionDate = &closed
				}
			} else {
				allCompleted = false
			}
		}

		rephrased := rephraseBatch(achievements)

		groupedTask := report.Task{
			ID:           listID,
			Title:        keyActivity,
			Description:  "",
			Achievements: strings.Join(rephrased, "\n"),
			Status:       "in progress",
			CreatedAt:    start,
			UpdatedAt:    latestUpdate,
			Source:       c.Name(),
			Type:         "Project",
		}

		if allCompleted && completionDate != nil {
			groupedTask.CompletedAt = completionDate
			groupedTask.Status = "completed"
		} else {
			//TODO: do better
			now := time.Now()
			groupedTask.CompletedAt = &now
		}

		groupedTasks = append(groupedTasks, groupedTask)
	}

	return groupedTasks, nil
}
