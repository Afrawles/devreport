package report

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

type ExcelExporter struct {
	OutputDir string
}

func NewExcelExporter(outputDir string) *ExcelExporter {
	return &ExcelExporter{OutputDir: outputDir}
}

func (e *ExcelExporter) Export(tasks []Task, start, end time.Time) error {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(e.OutputDir, fmt.Sprintf("summary_%s.xlsx", timestamp))

	f := excelize.NewFile()
	defer f.Close()

	projectTasks := make(map[string][]Task)
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

		projectTasks[project] = append(projectTasks[project], task)
	}

	sort.Strings(projectNames)

	if err := e.createDashboardSheet(f, "Dashboard", tasks, projectNames, start, end); err != nil {
		return fmt.Errorf("failed to create dashboard: %w", err)
	}

	for _, project := range projectNames {
		sheetName := sanitizeSheetName(project)
		if err := e.createProjectSheet(f, sheetName, projectTasks[project], start, end); err != nil {
			return fmt.Errorf("failed to create sheet for %s: %w", project, err)
		}
	}

	if err := f.DeleteSheet("Sheet1"); err != nil {
		//NOTE: 
	}

	if err := f.SaveAs(filename); err != nil {
		return fmt.Errorf("failed to save excel file: %w", err)
	}

	return nil
}

func (e *ExcelExporter) createDashboardSheet(f *excelize.File, sheetName string, tasks []Task, projectNames []string, start, end time.Time) error {
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}
	f.SetActiveSheet(index)

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

	for _, task := range tasks {
		project := task.Source
		if project == "" || project == "ClickUp" {
			project = "Unknown"
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

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
		Font: &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "#000000", Style: 1},
			{Type: "right", Color: "#000000", Style: 1},
			{Type: "top", Color: "#000000", Style: 1},
			{Type: "bottom", Color: "#000000", Style: 1},
		},
	})

	projectHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#B4C7E7"}, Pattern: 1},
		Font: &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "#000000", Style: 1},
			{Type: "right", Color: "#000000", Style: 1},
			{Type: "top", Color: "#000000", Style: 1},
			{Type: "bottom", Color: "#000000", Style: 1},
		},
	})

	totalStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#B4C7E7"}, Pattern: 1},
		Font: &excelize.Font{Bold: true},
		Border: []excelize.Border{
			{Type: "left", Color: "#000000", Style: 1},
			{Type: "right", Color: "#000000", Style: 1},
			{Type: "top", Color: "#000000", Style: 1},
			{Type: "bottom", Color: "#000000", Style: 1},
		},
	})

	f.SetCellValue(sheetName, "A1", "Date From:")
	f.SetCellValue(sheetName, "B1", start.Format("02-01-06"))
	f.SetCellValue(sheetName, "A2", "Date to:")
	f.SetCellValue(sheetName, "B2", end.Format("02-01-06"))

	row := 4

	col := 1
	f.SetCellValue(sheetName, cellName(col, row), "")
	f.SetCellStyle(sheetName, cellName(col, row), cellName(col, row), headerStyle)
	col++
	f.SetCellValue(sheetName, cellName(col, row), "Task Status")
	f.SetCellStyle(sheetName, cellName(col, row), cellName(col, row), headerStyle)
	col++

	for range projectNames {
		f.SetCellValue(sheetName, cellName(col, row), "Older Tasks")
		f.SetCellStyle(sheetName, cellName(col, row), cellName(col, row), headerStyle)
		col++
		f.SetCellValue(sheetName, cellName(col, row), "Reported This Week")
		f.SetCellStyle(sheetName, cellName(col, row), cellName(col, row), headerStyle)
		col++
		f.SetCellValue(sheetName, cellName(col, row), "All tasks")
		f.SetCellStyle(sheetName, cellName(col, row), cellName(col, row), headerStyle)
		col++
	}

	row++

	col = 1
	f.SetCellValue(sheetName, cellName(col, row), "")
	col++
	f.SetCellValue(sheetName, cellName(col, row), "")
	col++

	for _, project := range projectNames {
		f.SetCellValue(sheetName, cellName(col, row), project)
		f.SetCellStyle(sheetName, cellName(col, row), cellName(col+2, row), projectHeaderStyle)
		f.MergeCell(sheetName, cellName(col, row), cellName(col+2, row))
		col += 3
	}

	row++

	projectTotals := make(map[string]struct{ older, thisWeek, all int })
	for _, status := range statusOrder {
		col = 1
		f.SetCellValue(sheetName, cellName(col, row), "")
		col++
		f.SetCellValue(sheetName, cellName(col, row), status)
		col++

		for _, project := range projectNames {
			stats := projectData[project]
			older := stats.older[status]
			thisWeek := stats.thisWeek[status]
			all := stats.all[status]

			f.SetCellValue(sheetName, cellName(col, row), older)
			col++
			f.SetCellValue(sheetName, cellName(col, row), thisWeek)
			col++
			f.SetCellValue(sheetName, cellName(col, row), all)
			col++

			totals := projectTotals[project]
			totals.older += older
			totals.thisWeek += thisWeek
			totals.all += all
			projectTotals[project] = totals
		}
		row++
	}

	col = 1
	f.SetCellValue(sheetName, cellName(col, row), "")
	f.SetCellStyle(sheetName, cellName(col, row), cellName(col, row), totalStyle)
	col++
	f.SetCellValue(sheetName, cellName(col, row), "Total")
	f.SetCellStyle(sheetName, cellName(col, row), cellName(col, row), totalStyle)
	col++

	for _, project := range projectNames {
		totals := projectTotals[project]
		f.SetCellValue(sheetName, cellName(col, row), totals.older)
		f.SetCellStyle(sheetName, cellName(col, row), cellName(col, row), totalStyle)
		col++
		f.SetCellValue(sheetName, cellName(col, row), totals.thisWeek)
		f.SetCellStyle(sheetName, cellName(col, row), cellName(col, row), totalStyle)
		col++
		f.SetCellValue(sheetName, cellName(col, row), totals.all)
		f.SetCellStyle(sheetName, cellName(col, row), cellName(col, row), totalStyle)
		col++
	}

	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "B", 20)
	for i := 2; i < col; i++ {
		f.SetColWidth(sheetName, columnLetter(i), columnLetter(i), 15)
	}

	return nil
}

func (e *ExcelExporter) createProjectSheet(f *excelize.File, sheetName string, tasks []Task, start, end time.Time) error {
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}
	f.SetActiveSheet(index)

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
		Font: &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "#000000", Style: 1},
			{Type: "right", Color: "#000000", Style: 1},
			{Type: "top", Color: "#000000", Style: 1},
			{Type: "bottom", Color: "#000000", Style: 1},
		},
	})

	headers := []string{
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

	for col, header := range headers {
		cell := cellName(col+1, 1)
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	for i, task := range tasks {
		row := i + 2
		projectName := task.Source
		if projectName == "" || projectName == "ClickUp" {
			projectName = "Unknown"
		}

		f.SetCellValue(sheetName, cellName(1, row), i+1)
		f.SetCellValue(sheetName, cellName(2, row), task.Title)
		f.SetCellValue(sheetName, cellName(3, row), task.Assignee)
		f.SetCellValue(sheetName, cellName(4, row), normalizeStatus(task.Status))
		f.SetCellValue(sheetName, cellName(5, row), formatDate(task.CreatedAt))
		f.SetCellValue(sheetName, cellName(6, row), "") // Due date
		f.SetCellValue(sheetName, cellName(7, row), "") // Priority
		f.SetCellValue(sheetName, cellName(8, row), formatDatePtr(task.CompletedAt))
		f.SetCellValue(sheetName, cellName(9, row), projectName)
		f.SetCellValue(sheetName, cellName(10, row), task.Challenges)
		f.SetCellValue(sheetName, cellName(11, row), task.SupportRequired)
		f.SetCellValue(sheetName, cellName(12, row), task.SupportFrom)
		f.SetCellValue(sheetName, cellName(13, row), task.FollowUp)
	}

	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "B", 40)
	f.SetColWidth(sheetName, "C", "C", 20)
	f.SetColWidth(sheetName, "D", "D", 20)
	f.SetColWidth(sheetName, "E", "H", 15)
	f.SetColWidth(sheetName, "I", "I", 20)
	f.SetColWidth(sheetName, "J", "M", 20)

	f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	return nil
}

func cellName(col, row int) string {
	return fmt.Sprintf("%s%d", columnLetter(col), row)
}

func columnLetter(col int) string {
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}

func sanitizeSheetName(name string) string {
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, "?", "")
	name = strings.ReplaceAll(name, "*", "")
	name = strings.ReplaceAll(name, "[", "(")
	name = strings.ReplaceAll(name, "]", ")")

	if len(name) > 31 {
		name = name[:31]
	}

	return name
}
