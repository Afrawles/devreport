package report

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type CSVExporter struct {
	OutputDir string
}

func NewCSVExporter(outputDir string) *CSVExporter {
	return &CSVExporter{OutputDir: outputDir}
}

func (e *CSVExporter) Export(tasks []Task, start, end time.Time) error {
	if err := os.MkdirAll(e.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")

	if err := e.exportTaskList(tasks, timestamp, start, end); err != nil {
		return fmt.Errorf("failed to export task list: %w", err)
	}

	if err := e.exportDashboard(tasks, timestamp, start, end); err != nil {
		return fmt.Errorf("failed to export dashboard: %w", err)
	}

	return nil
}

func (e *CSVExporter) exportTaskList(tasks []Task, timestamp string, start, end time.Time) error {
	filename := filepath.Join(e.OutputDir, fmt.Sprintf("summary_%s_task_list.csv", timestamp))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"#",
		"Task Name",
		"Assignee",
		"Status",
		"Date Created",
		"Due Date",
		"Priority",
		"Date Cleared",
		"Project Name",
		"Challenges",
		"Support Required",
		"Support From",
		"Follow Up",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	for i, task := range tasks {
		projectName := task.Source
		if projectName == "" || projectName == "ClickUp" {
			projectName = extractProjectName(task.Title)
		}

		row := []string{
			fmt.Sprintf("%d", i+1),
			task.Title,
			task.Assignee,
			normalizeStatus(task.Status),
			formatDate(task.CreatedAt),
			"",
			"",
			formatDatePtr(task.CompletedAt),
			projectName,
			task.Challenges,
			task.SupportRequired,
			task.SupportFrom,
			task.FollowUp,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func (e *CSVExporter) exportDashboard(tasks []Task, timestamp string, start, end time.Time) error {
	filename := filepath.Join(e.OutputDir, fmt.Sprintf("summary_%s_dashboard.csv", timestamp))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	statusOrder := []string{
		"in progress",
		"issues",
		"improvements",
		"ready for qa",
		"ready for deployment",
		"failed qa",
		"backlog",
		"current sprint",
		"ready",
		"todo",
		"new development",
		"on hold",
		"urgent support",
		"suspended",
	}

	type ProjectStats struct {
		older    map[string]int
		thisWeek map[string]int
		all      map[string]int
	}

	projectData := make(map[string]*ProjectStats)
	projectNames := []string{}
	projectNameSet := make(map[string]bool)

	for _, task := range tasks {
		project := task.Source
		if project == "" || project == "ClickUp" {
			project = "Unknown"
		}

		if !projectNameSet[project] {
			projectNames = append(projectNames, project)
			projectNameSet[project] = true
		}

		if projectData[project] == nil {
			projectData[project] = &ProjectStats{
				older:    make(map[string]int),
				thisWeek: make(map[string]int),
				all:      make(map[string]int),
			}
		}

		status := strings.ToLower(strings.TrimSpace(task.Status))

		isThisWeek := task.CreatedAt.After(start) || task.CreatedAt.Equal(start)

		if isThisWeek {
			projectData[project].thisWeek[status]++
		} else {
			projectData[project].older[status]++
		}
		projectData[project].all[status]++
	}

	sort.Strings(projectNames)

	dateRow := []string{"Date From:", start.Format("02-01-06")}
	if err := writer.Write(dateRow); err != nil {
		return err
	}
	dateToRow := []string{"Date to:", end.Format("02-01-06")}
	if err := writer.Write(dateToRow); err != nil {
		return err
	}
	if err := writer.Write([]string{""}); err != nil {
		return err
	}

	header := []string{"", "Task Status"}
	for range projectNames {
		header = append(header, "Older Tasks", "Reported This Week", "All tasks")
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	projectRow := []string{"", ""}
	for _, project := range projectNames {
		projectRow = append(projectRow, project, "", "")
	}
	if err := writer.Write(projectRow); err != nil {
		return err
	}

	projectTotals := make(map[string]struct{ older, thisWeek, all int })
	for _, status := range statusOrder {
		row := []string{"", normalizeStatusDisplay(status)}

		for _, project := range projectNames {
			stats := projectData[project]
			older := stats.older[status]
			thisWeek := stats.thisWeek[status]
			all := stats.all[status]

			row = append(row,
				fmt.Sprintf("%d", older),
				fmt.Sprintf("%d", thisWeek),
				fmt.Sprintf("%d", all),
			)

			totals := projectTotals[project]
			totals.older += older
			totals.thisWeek += thisWeek
			totals.all += all
			projectTotals[project] = totals
		}

		if err := writer.Write(row); err != nil {
			return err
		}
	}

	totalsRow := []string{"", "Total"}
	for _, project := range projectNames {
		totals := projectTotals[project]
		totalsRow = append(totalsRow,
			fmt.Sprintf("%d", totals.older),
			fmt.Sprintf("%d", totals.thisWeek),
			fmt.Sprintf("%d", totals.all),
		)
	}
	if err := writer.Write(totalsRow); err != nil {
		return err
	}

	return nil
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02/01/06")
}

func formatDatePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("02/01/06")
}

func normalizeStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	status = strings.ReplaceAll(status, "_", " ")
	return status
}

func normalizeStatusDisplay(status string) string {
	return strings.TrimSpace(status)
}

func extractProjectName(title string) string {
	for _, sep := range []string{" - ", ": ", " | "} {
		if idx := strings.Index(title, sep); idx > 0 {
			return strings.TrimSpace(title[:idx])
		}
	}
	
	parts := strings.Fields(title)
	if len(parts) > 0 {
		return parts[0]
	}
	
	return "Unknown"
}
