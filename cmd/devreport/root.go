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

	"github.com/schollz/progressbar/v3"
)

var (
	startDate        string
	endDate          string
	username         string
	output           string
	clickUpToken     string
	clickUpAssignees string
	clickupListIDs   string
	clickupFolderID  string
	author           string
	category         string
	challenges       string
	supportRequired  string
	supportFrom      string
	followUp         string
	period           string
	year             int
	csvOutput        string
)

var rootCmd = &cobra.Command{
	Use:   "devreport",
	Short: "Generate developer activity reports for HR",
	Long:  `DevReport aggregates developer activities from GitHub, ClickUp etc.`,
	Run:   generateReport,
}

var (
	summaryCmd = &cobra.Command{
		Use:   "summary",
		Short: "Generate team-wide CSV summary reports (for ops/business)",
		Long:  `Generates clean CSV reports with task list + dashboard for the whole team.`,
		Run:   generateSummary,
	}

	periodFlag string
)

func execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// godotenv.Load()
	rootCmd.AddCommand(summaryCmd)

	rootCmd.Flags().StringVarP(&startDate, "start", "s", "", "Start date (YYYY-MM-DD)")
	rootCmd.Flags().StringVarP(&endDate, "end", "e", "", "End date (YYYY-MM-DD)")
	rootCmd.Flags().StringVarP(&username, "user", "u", "", "Username to generate report for")
	rootCmd.Flags().StringVarP(&output, "output", "o", "reports", "Output directory")

	// clickup
	rootCmd.Flags().StringVar(&clickUpToken, "clickup-token", "", "ClickUp API token")
	rootCmd.Flags().StringVar(&clickUpAssignees, "clickup-assignees", "", "Comma-separated ClickUp assignee IDs")
	rootCmd.Flags().StringVar(&clickupListIDs, "clickup-listid", "", "CLickup List IDs")
	rootCmd.Flags().StringVar(&clickupFolderID, "clickup-folderid", "", "ClickUp Folder ID (alternative to list IDs)")

	rootCmd.Flags().StringVar(&category, "category", "Improvements/Issues/New Development/Urgent Support/Fixes", "Category suffix for list names")

	rootCmd.Flags().StringVar(&author, "author", "", "report author")

	rootCmd.Flags().StringVar(&challenges, "challenges", "", "Comma-separated challenges encountered (one per list)")
	rootCmd.Flags().StringVar(&supportRequired, "support-required", "", "Comma-separated support required (one per list)")
	rootCmd.Flags().StringVar(&supportFrom, "support-from", "", "Comma-separated support from (one per list)")
	rootCmd.Flags().StringVar(&followUp, "follow-up", "", "Comma-separated follow up activities (one per list)")
	rootCmd.Flags().StringVar(&period, "period", "Q2", "Reporting period (e.g., Q1, Q2, January, etc.)")
	rootCmd.Flags().IntVar(&year, "year", time.Now().Year(), "Report year")

	rootCmd.Flags().StringVar(&csvOutput, "csv", "", "Generate CSV summary reports (directory or filename prefix)")

	summaryCmd.Flags().StringVar(&periodFlag, "period", "this-week", "Period: today, yesterday, this-week, last-week, this-month, last-month, all-time")
	summaryCmd.Flags().StringVar(&clickUpToken, "clickup-token", "", "ClickUp API token")
	summaryCmd.Flags().StringVar(&clickupListIDs, "clickup-listid", "", "ClickUp List IDs (comma-separated)")
	summaryCmd.Flags().StringVar(&clickupFolderID, "clickup-folderid", "", "ClickUp Folder ID (fetches all lists in folder)")
	summaryCmd.Flags().StringVar(&clickUpAssignees, "clickup-assignees", "", "Filter by assignee IDs (optional)")
	summaryCmd.Flags().StringVar(&csvOutput, "csv", "reports", "Output directory for CSV reports")
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

	fmt.Printf("Generating report for %s (%s to %s)\n",
		username, start.Format("2006-01-02"), end.Format("2006-01-02"))

	var sources []report.ActivitySource

	// clickUp
	token := clickUpToken
	if token == "" {
		token = os.Getenv("CLICKUP_API_KEY")
	}

	listIDstr := clickupListIDs
	if listIDstr == "" {
		listIDstr = os.Getenv("CLICKUP_LISTIDS")
	}

	folderID := clickupFolderID
	if folderID == "" {
		folderID = os.Getenv("CLICKUP_FOLDERID")
	}

	assigneesStr := clickUpAssignees
	if assigneesStr == "" {
		assigneesStr = os.Getenv("CLICKUP_ASSIGNEE_IDS")
	}

	if token != "" && assigneesStr != "" {
		assigneeIDs := strings.Split(assigneesStr, ",")
		for i := range assigneeIDs {
			assigneeIDs[i] = strings.TrimSpace(assigneeIDs[i])
		}

		var listIDs []string

		if folderID != "" {
			bar := newSpinner("Fetching lists from folder")
			defer finishBar(bar)

			client := clickup.NewClient(token, nil, assigneeIDs)
			listIDs, err = client.GetListIDsFromFolder(folderID)

			if err != nil {
				fmt.Printf("\nError fetching lists from folder: %v\n", err)
				return
			}
			fmt.Printf("Found %d lists in folder\n\n", len(listIDs))
		} else if listIDstr != "" {
			listIDs = strings.Split(listIDstr, ",")
			for i := range listIDs {
				listIDs[i] = strings.TrimSpace(listIDs[i])
			}
		}

		if len(listIDs) > 0 {
			sources = append(sources, clickup.NewClickUpSource(token, listIDs, assigneeIDs, category))
		} else {
			fmt.Println("No list IDs found. Provide --clickup-listid or --clickup-folderid")
			return
		}
	} else if token != "" {
		fmt.Println("ClickUp token provided but assignee IDs missing")
	}

	if len(sources) == 0 {
		fmt.Println("No data sources configured. Set tokens via flags or environment variables.")
		fmt.Println("Required: CLICKUP_API_KEY + CLICKUP_ASSIGNEE_IDS + (CLICKUP_LISTIDS or CLICKUP_FOLDERID)")
		return
	}

	// progress bar
	bar := newSpinner("Fetching tasks")
	defer finishBar(bar)

	gen := report.NewGenerator(sources...)
	tasks, err := gen.Generate(context.Background(), username, start, end)

	if err != nil {
		fmt.Printf("\nError generating report: %v\n", err)
		return
	}

	if len(tasks) == 0 {
		fmt.Println("\nNo activities found for this period")
		return
	}

	fmt.Printf("Fetched %d tasks\n\n", len(tasks))

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

	err = os.MkdirAll(output, 0755)
	if err != nil {
		return
	}

	exporter := report.NewExporter(output)
	stats := gen.Statistics(tasks)

	fmt.Println("Generating reports...")
	exportBar := progressbar.NewOptions(3,
		progressbar.OptionSetDescription("Exporting"),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
	)
	defer finishBar(exportBar)

	// json
	jsonFile := fmt.Sprintf("report_%s_%s.json", username, time.Now().Format("20060102_150405"))
	if err := exporter.ExportJSON(tasks, jsonFile); err != nil {
		fmt.Printf("Failed to export JSON: %v\n", err)
	} else {
		_ = exportBar.Add(1)
	}

	//html
	htmlFile := fmt.Sprintf("report_%s_%s.html", username, time.Now().Format("20060102_150405"))
	reportConfig := map[string]any{
		"Year":   year,
		"Period": period,
	}
	if err := exporter.ExportHTML(tasks, stats, htmlFile, author, reportConfig); err != nil {
		fmt.Printf("Failed to export html: %v\n", err)
	} else {
		_ = exportBar.Add(1)
	}

	// csv
	if csvOutput != "" {
		csvExporter := report.NewCSVExporter(csvOutput)
		if err := csvExporter.Export(tasks, start, end); err != nil {
			fmt.Printf("Failed to export CSV: %v\n", err)
		} else {
			_ = exportBar.Add(1)
		}
	} else {
		_ = exportBar.Add(1)
	}

	fmt.Printf("\nReports saved to %s/\n", output)
	fmt.Printf("  -> %s (JSON)\n", jsonFile)
	fmt.Printf("  -> %s (HTML)\n", htmlFile)
	if csvOutput != "" {
		fmt.Printf("  -> CSV reports in %s/\n", csvOutput)
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Total activities: %d\n", stats["total"])
	fmt.Printf("  Completed: %d\n", stats["completed"])
}

func generateSummary(cmd *cobra.Command, args []string) {
	token := clickUpToken
	if token == "" {
		token = os.Getenv("CLICKUP_API_KEY")
	}

	folderID := clickupFolderID
	if folderID == "" {
		folderID = os.Getenv("CLICKUP_FOLDERID")
	}

	var listIDs []string
	var err error
	var listNames map[string]string

	if folderID != "" {
		bar := newSpinner("Fetching lists from folder")
		defer finishBar(bar)

		client := clickup.NewClient(token, nil, nil)
		listIDs, listNames, err = client.GetListIDsAndNamesFromFolder(folderID)

		if err != nil {
			fmt.Printf("\nError fetching lists from folder: %v\n", err)
			return
		}
		fmt.Printf("Found %d lists in folder\n\n", len(listIDs))
	} else if clickupListIDs != "" {
		listIDs = strings.Split(clickupListIDs, ",")
		for i := range listIDs {
			listIDs[i] = strings.TrimSpace(listIDs[i])
		}
		listNames = make(map[string]string)
	}

	if token == "" || len(listIDs) == 0 {
		fmt.Println("Error: --clickup-token and (--clickup-listid or --clickup-folderid) are required")
		fmt.Println("Or set CLICKUP_API_KEY and (CLICKUP_LISTIDS or CLICKUP_FOLDERID) env vars")
		return
	}

	now := time.Now()
	var start, end time.Time

	switch strings.ToLower(periodFlag) {
	case "today":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = start.Add(24 * time.Hour)
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		start = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
		end = start.Add(24 * time.Hour)
	case "this-week", "thisweek":
		daysSinceMonday := int(now.Weekday() - time.Monday)
		if daysSinceMonday < 0 {
			daysSinceMonday += 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-daysSinceMonday, 0, 0, 0, 0, now.Location())
		end = start.Add(7 * 24 * time.Hour)
	case "last-week", "lastweek":
		daysSinceMonday := int(now.Weekday() - time.Monday)
		if daysSinceMonday < 0 {
			daysSinceMonday += 7
		}
		monday := now.AddDate(0, 0, -daysSinceMonday-7)
		start = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, monday.Location())
		end = start.Add(7 * 24 * time.Hour)
	case "this-month", "thismonth":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 1, 0)
	case "last-month", "lastmonth":
		firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		start = firstOfThisMonth.AddDate(0, -1, 0)
		end = firstOfThisMonth
	case "all-time", "alltime":
		start = time.Time{}
		end = now.Add(24 * time.Hour)
	default:
		fmt.Printf("Unknown period: %s\n", periodFlag)
		fmt.Println("Valid options: today, yesterday, this-week, last-week, this-month, last-month, all-time")
		return
	}

	fmt.Printf("Generating team summary for: %s -> %s\n", start.Format("2006-01-02"), end.Format("2006-01-02"))

	var assigneeIDs []string
	if clickUpAssignees != "" {
		assigneeIDs = strings.Split(clickUpAssignees, ",")
		for i := range assigneeIDs {
			assigneeIDs[i] = strings.TrimSpace(assigneeIDs[i])
		}
	}
	source := clickup.NewClickUpSource(token, listIDs, assigneeIDs, "")

	source.Client.SetListNames(listNames)

	bar := newSpinner("Fetching tasks")
	defer finishBar(bar)

	gen := report.NewGenerator(source)
	tasks, err := gen.Generate(context.Background(), "", start, end)

	if err != nil {
		fmt.Printf("\nFailed to fetch tasks: %v\n", err)
		return
	}

	fmt.Printf("Found %d tasks\n\n", len(tasks))

	exportBar := newSpinner("Generating CSV reports")
	defer finishBar(exportBar)

	// csvExporter := report.NewCSVExporter(csvOutput)
	// if err := csvExporter.Export(tasks, start, end); err != nil {
	// 	fmt.Printf("\nCSV export failed: %v\n", err)
	// 	return
	// }

	// fmt.Println("\nSummary reports ready for business team!")
	// fmt.Printf("  -> %s/summary_*_task_list.csv\n", csvOutput)
	// fmt.Printf("  -> %s/summary_*_dashboard.csv\n", csvOutput)
	//
	excelExporter := report.NewExcelExporter(csvOutput)
	if err := excelExporter.Export(tasks, start, end); err != nil {
		fmt.Printf("\nExcel export failed: %v\n", err)
		return
	}

	fmt.Println("\nSummary report ready for business team!")
	fmt.Printf("  -> %s/summary_*.xlsx (with Dashboard + sheets per project)\n", csvOutput)
}

func newSpinner(description string) *progressbar.ProgressBar {
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetWidth(15),
		progressbar.OptionThrottle(100*time.Millisecond),
	)
	_ = bar.RenderBlank()
	return bar
}

func finishBar(bar *progressbar.ProgressBar) {
	if bar != nil {
		_ = bar.Finish()
	}
}
