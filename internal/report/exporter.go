package report

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sort"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

//go:embed "templates"
var templateFS embed.FS

type Exporter struct {
	OutputDir string
}

func NewExporter(outputDir string) *Exporter {
	return &Exporter{OutputDir: outputDir}
}

func (e *Exporter) ExportJSON(tasks []Task, filename string) error {
	data, err := json.MarshalIndent(tasks, "", "\t")
	if err != nil {
		return err
	}

	return os.WriteFile(fmt.Sprintf("%s/%s", e.OutputDir, filename), data, 0644)
}

func (e *Exporter) ExportHTML(tasks []Task, stats map[string]any, filename, author string, config map[string]any) error {
	funcMap := template.FuncMap{
		"title": cases.Title(language.English).String,
		"sub":   func(a, b int) int { return a - b },
	}
	tmpl, err := template.New("report.tmpl").Funcs(funcMap).ParseFS(templateFS, "templates/report.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}

	outputPath := fmt.Sprintf("%s/%s", e.OutputDir, filename)
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create HTML file: %w", err)
	}
	defer f.Close()

	year := time.Now().Year()
	period := time.Now().Format("January")
	if config != nil {
		if y, ok := config["Year"].(int); ok {
			year = y
		}
		if p, ok := config["Period"].(string); ok {
			period = p
		}
	}

	tasksByProject := make(map[string][]Task)
	for _, task := range tasks {
		source := task.Source
		if source == "" {
			source = "Uncategorized"
		}
		tasksByProject[source] = append(tasksByProject[source], task)
	}
	
	type ProjectGroup struct {
		ProjectName string
		Tasks       []Task
	}
	
	var groupedTasks []ProjectGroup
	for projectName, projectTasks := range tasksByProject {
		groupedTasks = append(groupedTasks, ProjectGroup{
			ProjectName: projectName,
			Tasks:       projectTasks,
		})
	}
	
	sort.Slice(groupedTasks, func(i, j int) bool {
		return groupedTasks[i].ProjectName < groupedTasks[j].ProjectName
	})

	data := map[string]any{
		"Date":        time.Now().Format("2006-01-02 15:04:05"),
		"Tasks":       tasks,
		"GroupedTasks": groupedTasks,
		"Stats":       stats,
		"Year":        year,
		"Department":  "Information Systems",
		"SubmittedBy": author,
		"Period":      period,
	}

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to render HTML: %w", err)
	}

	fmt.Printf("HTML report saved: %s\n", outputPath)
	return nil
}
