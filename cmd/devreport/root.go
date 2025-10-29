package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Afrawles/devreport/internal/clickup"
	"github.com/Afrawles/devreport/internal/report"
	"github.com/spf13/cobra"
)

var (
    startDate string
    endDate   string
    username  string
    output    string
	clickUpToken    string
	clickUpAssignees string
	clickuoListID string
)

var rootCmd = &cobra.Command{
    Use:   "devreport",
    Short: "Generate developer activity reports for HR",
    Long:  `DevReport aggregates developer activities from GitHub, ClickUp etc.`,
    Run:   generateReport,
}

func execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func init() {
    // godotenv.Load()

    rootCmd.Flags().StringVarP(&startDate, "start", "s", "", "Start date (YYYY-MM-DD)")
    rootCmd.Flags().StringVarP(&endDate, "end", "e", "", "End date (YYYY-MM-DD)")
    rootCmd.Flags().StringVarP(&username, "user", "u", "", "Username to generate report for")
    rootCmd.Flags().StringVarP(&output, "output", "o", "reports", "Output directory")

	// clickup 
	rootCmd.Flags().StringVar(&clickUpToken, "clickup-token", "", "ClickUp API token")
	rootCmd.Flags().StringVar(&clickUpAssignees, "clickup-assignees", "", "Comma-separated ClickUp assignee IDs")
	rootCmd.Flags().StringVar(&clickuoListID, "clickup-listid", "", "CLickup List ID")
}


func generateReport(cmd *cobra.Command, args []string) {
    var start, end time.Time
    var err error

    if startDate == "" {
        start = time.Now().AddDate(0, 0, -7)
    } else {
        start, err = time.Parse("2006-01-02", startDate)
        if err != nil {
            fmt.Printf("Invalid start date: %v\n", err)
            return
        }
    }

    if endDate == "" {
        end = time.Now()
    } else {
        end, err = time.Parse("2006-01-02", endDate)
        if err != nil {
            fmt.Printf("Invalid end date: %v\n", err)
            return
        }
    }

    if username == "" {
        fmt.Println("Username is required. Use --user flag")
        return
    }

    fmt.Printf("generating report for %s (%s to %s)\n\n", 
        username, start.Format("2006-01-02"), end.Format("2006-01-02"))

    var sources []report.ActivitySource

	// clickUp
	token := clickUpToken
	if token == "" {
		token = os.Getenv("CLICKUP_API_KEY")
	}

	listID := clickuoListID
	if listID == "" {
		listID = os.Getenv("CLICKUP_LISTID")
	}

	assigneesStr := clickUpAssignees
	if assigneesStr == "" {
		assigneesStr = os.Getenv("CLICKUP_ASSIGNEE_IDS")
	}

	if token != "" && assigneesStr != ""  && listID != ""{
		assigneeIDs := strings.Split(assigneesStr, ",")
		for i := range assigneeIDs {
			assigneeIDs[i] = strings.TrimSpace(assigneeIDs[i])
		}
		sources = append(sources, clickup.NewClickUpSource(token, listID, assigneeIDs))
	} else if token != "" {
		fmt.Println("ClickUp token provided but assignee IDs missing")
	}

	if len(sources) == 0 {
		fmt.Println("No data sources configured. Set tokens via flags or environment variables.")
		fmt.Println("Required: CLICKUP_API_KEY + CLICKUP_ASSIGNEE_IDS")
		return
	}

    gen := report.NewGenerator(sources...)
    tasks, err := gen.Generate(context.Background(), username, start, end)
    if err != nil {
        fmt.Printf("Error generating report: %v\n", err)
        return
    }

    if len(tasks) == 0 {
        fmt.Println("No activities found for this period")
        return
    }

    os.MkdirAll(output, 0755)

    exporter := report.NewExporter(output)
    stats := gen.Statistics(tasks)

	// json
    jsonFile := fmt.Sprintf("report_%s_%s.json", username, time.Now().Format("20060102"))
    if err := exporter.ExportJSON(tasks, jsonFile); err != nil {
        fmt.Printf("Failed to export JSON: %v\n", err)
    } else {
        fmt.Printf("JSON report saved: %s/%s\n", output, jsonFile)
    }

    //html
    htmlFile := fmt.Sprintf("report_%s_%s.html", username, time.Now().Format("20060102"))
    if err := exporter.ExportHTML(tasks, stats, htmlFile); err != nil {
        fmt.Printf("Failed to export html: %v\n", err)
    } else {
        fmt.Printf("html report saved: %s/%s\n", output, htmlFile)
    }

    fmt.Printf("\nSummary:\n")
    fmt.Printf("   Total activities: %d\n", stats["total"])
    fmt.Printf("   Completed: %d\n", stats["completed"])
}

