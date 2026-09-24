package clickup

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Afrawles/devreport/internal/llm"
	"github.com/Afrawles/devreport/internal/report"
)

const defaultOllamaModel = "mistral-nemo:latest"

type ClickUpSource struct {
	Client   *Client
	Category string
	LLM      llm.Provider
}

func NewClickUpSource(apiKey string, listID, assigneeIDs []string, category string, llmCfg llm.Config) *ClickUpSource {
	llmCfg.OllamaModel = defaultOllamaModel
	return &ClickUpSource{
		Client:   NewClient(apiKey, listID, assigneeIDs),
		Category: category,
		LLM:      llm.New(llmCfg),
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

		rephrased := rephraseTask(c.LLM, t.Description)
		task := report.Task{
			ID:              t.ID,
			Title:           t.Name,
			Description:     t.Description,
			Achievements:    rephrased,
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

func rephraseTask(p llm.Provider, taskDescription string) string {
	if strings.TrimSpace(taskDescription) == "" {
		return taskDescription
	}

	prompt := "Rephrase the following task description as a concise, professional achievement bullet point.\n\n" +
		"STRICT RULES:\n" +
		"1. Use strong action verbs and focus on the accomplishment\n" +
		"2. For currency: Add 'UGX' prefix to numbers that represent money (e.g., '5000' becomes 'UGX 5000')\n" +
		"3. PRESERVE all numerical values EXACTLY as written - do not modify, round, or change any numbers\n" +
		"4. Only fix spelling errors and grammar mistakes\n" +
		"5. Do NOT change the core meaning or description of the task\n" +
		"6. Return only the rephrased text without bullet point symbols (•, -, *)\n\n" +
		"Original description:\n" +
		taskDescription

	rephrased, err := p.Complete(prompt)
	if err != nil {
		fmt.Printf("%s unavailable for task rephrase: %v\n", p.Name(), err)
		return taskDescription
	}

	return rephrased
}
