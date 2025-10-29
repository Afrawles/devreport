package devreport

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/Afrawles/devreport/internal/clickup"
	"github.com/Afrawles/devreport/internal/config"
	"github.com/Afrawles/devreport/internal/report"
)

type Application struct {
    Config    *config.Config
    Logger    *slog.Logger
    Generator *report.Generator
    Exporter  *report.Exporter
    Wg        sync.WaitGroup
}

func New(cfg *config.Config) *Application {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    slog.SetDefault(logger)

    var sources []report.ActivitySource

    if cfg.ClickUp.APIKey != "" && len(cfg.ClickUp.AssigneeIDs) > 0 {
        sources = append(sources, clickup.NewClickUpSource(cfg.ClickUp.APIKey, cfg.ClickUp.ListID, cfg.ClickUp.AssigneeIDs))
        logger.Info("ClickUp source initialized", "assignees", len(cfg.ClickUp.AssigneeIDs))
    }

    generator := report.NewGenerator(sources...)
    exporter := report.NewExporter(cfg.Output.Directory)

    return &Application{
        Config:    cfg,
        Logger:    logger,
        Generator: generator,
        Exporter:  exporter,
    }
}

func (app *Application) GenerateReport(ctx context.Context, user string, start, end time.Time) error {
    app.Logger.Info("generating report",
        "user", user,
        "start", start.Format("2006-01-02"),
        "end", end.Format("2006-01-02"),
    )

    tasks, err := app.Generator.Generate(ctx, user, start, end)
    if err != nil {
        app.Logger.Error("failed to generate report", "error", err)
        return err
    }

    if len(tasks) == 0 {
        app.Logger.Warn("no activities found for this period")
        return nil
    }

    app.Logger.Info("tasks fetched", "count", len(tasks))

    if err := os.MkdirAll(app.Config.Output.Directory, 0755); err != nil {
        return fmt.Errorf("failed to create output directory: %w", err)
    }

    stats := app.Generator.Statistics(tasks)

    timestamp := time.Now().Format("20060102")

    for _, format := range app.Config.Output.Format {
        switch format {
        case "json":
            filename := fmt.Sprintf("report_%s_%s.json", user, timestamp)
            if err := app.Exporter.ExportJSON(tasks, filename); err != nil {
                app.Logger.Error("failed to export JSON", "error", err)
            } else {
                app.Logger.Info("report exported", "format", "json", "file", filename)
            }

        case "html":
            filename := fmt.Sprintf("report_%s_%s.md", user, timestamp)
            if err := app.Exporter.ExportHTML(tasks, stats, filename); err != nil {
                app.Logger.Error("failed to export Markdown", "error", err)
            } else {
                app.Logger.Info("report exported", "format", "markdown", "file", filename)
            }
        }
    }

    app.Logger.Info("report generation complete",
        "total", stats["total"],
        "completed", stats["completed"],
    )

    return nil
}
