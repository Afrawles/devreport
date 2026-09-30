package report

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"time"
)

// LoadTasksFromJSON reads a []Task previously written by Exporter.ExportJSON.
func LoadTasksFromJSON(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// LoadTasksFromCSV reads tasks from a CSV with a header row. Column names are
// matched case-insensitively against a few accepted aliases, so it accepts
// both a hand-built export and similarly-shaped CSVs from other tools.
func LoadTasksFromCSV(path string) ([]Task, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	col := func(names ...string) int {
		for i, h := range rows[0] {
			hn := strings.ToLower(strings.TrimSpace(h))
			for _, n := range names {
				if hn == n {
					return i
				}
			}
		}
		return -1
	}

	idIdx := col("id", "prnumber", "pr number", "#")
	titleIdx := col("title", "task name")
	statusIdx := col("status")
	urlIdx := col("url")
	sourceIdx := col("source", "repo", "project name")
	createdIdx := col("createdat", "date created")
	completedIdx := col("completedat", "closedat", "date cleared")
	achievementIdx := col("achievements", "achievement")

	get := func(row []string, idx int) string {
		if idx < 0 || idx >= len(row) {
			return ""
		}
		return row[idx]
	}

	parseTime := func(s string) time.Time {
		if s == "" {
			return time.Time{}
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02", "02/01/06"} {
			if t, err := time.Parse(layout, s); err == nil {
				return t
			}
		}
		return time.Time{}
	}

	var tasks []Task
	for _, row := range rows[1:] {
		createdAt := parseTime(get(row, createdIdx))

		var completedAt *time.Time
		if s := get(row, completedIdx); s != "" {
			if t := parseTime(s); !t.IsZero() {
				completedAt = &t
			}
		}

		tasks = append(tasks, Task{
			ID:           get(row, idIdx),
			Title:        get(row, titleIdx),
			Status:       get(row, statusIdx),
			URL:          get(row, urlIdx),
			Source:       get(row, sourceIdx),
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
			CompletedAt:  completedAt,
			Achievements: get(row, achievementIdx),
			Type:         "Pull Request",
		})
	}

	return tasks, nil
}
