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
	startDate   string
	endDate     string
	username    string
	output      string
	clickUpToken    string
	clickUpAssignees string
	clickuoListIDs string
	author string
	category string
	challenges      string
	supportRequired string
	supportFrom     string
	followUp        string
	period          string
	year            int
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
	rootCmd.Flags().StringVar(&clickuoListIDs, "clickup-listid", "", "CLickup List IDs")
	rootCmd.Flags().StringVar(&category, "category", "Improvements/Issues/New Development/Urgent Support/Fixes", "Category suffix for list names")

	rootCmd.Flags().StringVar(&author, "author", "", "report author")

	rootCmd.Flags().StringVar(&challenges, "challenges", "", "Comma-separated challenges encountered (one per list)")
	rootCmd.Flags().StringVar(&supportRequired, "support-required", "", "Comma-separated support required (one per list)")
	rootCmd.Flags().StringVar(&supportFrom, "support-from", "", "Comma-separated support from (one per list)")
	rootCmd.Flags().StringVar(&followUp, "follow-up", "", "Comma-separated follow up activities (one per list)")
	rootCmd.Flags().StringVar(&period, "period", "Q2", "Reporting period (e.g., Q1, Q2, January, etc.)")
	rootCmd.Flags().IntVar(&year, "year", time.Now().Year(), "Report year")
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

	listIDstr := clickuoListIDs
	if listIDstr == "" {
		listIDstr = os.Getenv("CLICKUP_LISTIDS")
	}

	assigneesStr := clickUpAssignees
	if assigneesStr == "" {
		assigneesStr = os.Getenv("CLICKUP_ASSIGNEE_IDS")
	}

	if token != "" && assigneesStr != ""  && listIDstr != ""{
		assigneeIDs := strings.Split(assigneesStr, ",")
		for i := range assigneeIDs {
			assigneeIDs[i] = strings.TrimSpace(assigneeIDs[i])
		}
		listIDs := strings.Split(listIDstr, ",")
		for i := range listIDs {
			listIDs[i] = strings.TrimSpace(listIDs[i])
		}
		sources = append(sources, clickup.NewClickUpSource(token, listIDs, assigneeIDs, category))
	} else if token != "" {
		fmt.Println("ClickUp token provided but assignee IDs missing")
	}

	if len(sources) == 0 {
		fmt.Println("No data sources configured. Set tokens via flags or environment variables.")
		fmt.Println("Required: CLICKUP_API_KEY + CLICKUP_ASSIGNEE_IDS + CLICKUP_LISTIDS")
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

	challengesList := parseCommaList(challenges)
	supportRequiredList := parseCommaList(supportRequired)
	supportFromList := parseCommaList(supportFrom)
	followUpList := parseCommaList(followUp)

	for i := range tasks {
		if i < len(challengesList) {
			tasks[i].Challenges = challengesList[i]
		}
		if i < len(supportRequiredList) {
			tasks[i].SupportRequired = supportRequiredList[i]
		}
		if i < len(supportFromList) {
			tasks[i].SupportFrom = supportFromList[i]
		}
		if i < len(followUpList) {
			tasks[i].FollowUp = followUpList[i]
		}
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
	reportConfig := map[string]any{
		"Year":   year,
		"Period": period,
	}
	if err := exporter.ExportHTML(tasks, stats, htmlFile, author, reportConfig); err != nil {
		fmt.Printf("Failed to export html: %v\n", err)
	} else {
		fmt.Printf("html report saved: %s/%s\n", output, htmlFile)
	}

    fmt.Printf("\nSummary:\n")
    fmt.Printf("   Total activities: %d\n", stats["total"])
    fmt.Printf("   Completed: %d\n", stats["completed"])
}

