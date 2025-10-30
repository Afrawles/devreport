package report

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
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

func (e *Exporter) ExportHTML(tasks []Task, stats map[string]any, filename,  auhtor string) error {
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

    data := map[string]any{
        "Date":        time.Now().Format("2006-01-02 15:04:05"),
        "Tasks":       tasks,
        "Stats":       stats,
        "Year":        2025,
        "Department":  "Information Systems",
        "SubmittedBy": auhtor,
        "Period":      "Q2",
    }

	fmt.Println(stats)

    if err := tmpl.Execute(f, data); err != nil {
        return fmt.Errorf("failed to render HTML: %w", err)
    }

    fmt.Printf("HTML report saved: %s\n", outputPath)
    return nil
}
