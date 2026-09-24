package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Afrawles/devreport/internal/llm"
	"github.com/Afrawles/devreport/internal/report"
	"github.com/spf13/cobra"
)

var (
	clusterInput       string
	clusterOutput      string
	clusterMinGroup    int
	clusterBatch       int
	clusterAuthor      string
	clusterOllamaModel string
)

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Merge related/small tasks in an existing report into combined rows",
	Long: `Reads a previously exported report (JSON from a normal devreport run, or a
CSV with Title/Status/Source/Achievements columns), groups tasks per source,
and asks an LLM to combine related or small tasks into fewer, still-technical
rows. Use this after a fetch when the report came out too long to review.`,
	Run: generateCluster,
}

func init() {
	rootCmd.AddCommand(clusterCmd)

	clusterCmd.Flags().StringVar(&clusterInput, "input", "", "Path to a report .json or .csv file to cluster (required)")
	clusterCmd.Flags().StringVar(&clusterOutput, "output", "reports", "Output directory for the clustered report")
	clusterCmd.Flags().IntVar(&clusterMinGroup, "min-group", 5, "Skip clustering for a source with fewer than this many tasks")
	clusterCmd.Flags().IntVar(&clusterBatch, "batch-size", 40, "Max tasks sent to the LLM per clustering call")
	clusterCmd.Flags().StringVar(&clusterAuthor, "author", "", "Report author name for the HTML export")

	clusterCmd.Flags().StringVar(&llmProvider, "llm-provider", "ollama", "LLM backend for clustering: ollama or claude")
	clusterCmd.Flags().StringVar(&claudeAPIKey, "claude-api-key", "", "Anthropic API key (or ANTHROPIC_API_KEY env var)")
	clusterCmd.Flags().StringVar(&claudeModel, "claude-model", "", "Anthropic model id (default: claude-sonnet-5)")
	clusterCmd.Flags().StringVar(&clusterOllamaModel, "ollama-model", "gemma4:e4b", "Ollama model id to use for clustering")
}

func generateCluster(cmd *cobra.Command, args []string) {
	if clusterInput == "" {
		fmt.Println("Input file required. Use --input <report.json|report.csv>")
		return
	}

	var tasks []report.Task
	var err error

	switch strings.ToLower(filepath.Ext(clusterInput)) {
	case ".json":
		tasks, err = report.LoadTasksFromJSON(clusterInput)
	case ".csv":
		tasks, err = report.LoadTasksFromCSV(clusterInput)
	default:
		fmt.Println("Unsupported input file type. Use a .json or .csv file")
		return
	}
	if err != nil {
		fmt.Printf("Failed to load %s: %v\n", clusterInput, err)
		return
	}
	if len(tasks) == 0 {
		fmt.Println("No tasks found in input file")
		return
	}

	fmt.Printf("Loaded %d tasks from %s\n", len(tasks), clusterInput)

	llmCfg := resolveLLMConfig(cmd)
	llmCfg.OllamaModel = clusterOllamaModel
	provider := llm.New(llmCfg)
	clusterer := report.NewClusterer(provider, clusterBatch, clusterMinGroup)

	clustered := clusterer.ClusterBySource(tasks)
	fmt.Printf("Clustered %d tasks into %d rows using %s\n", len(tasks), len(clustered), provider.Name())

	if err := os.MkdirAll(clusterOutput, 0755); err != nil {
		fmt.Printf("Failed to create output directory: %v\n", err)
		return
	}

	gen := report.NewGenerator()
	stats := gen.Statistics(clustered)
	exporter := report.NewExporter(clusterOutput)
	timestamp := time.Now().Format("20060102_150405")

	jsonFile := fmt.Sprintf("clustered_%s.json", timestamp)
	if err := exporter.ExportJSON(clustered, jsonFile); err != nil {
		fmt.Printf("Failed to export JSON: %v\n", err)
	}

	htmlFile := fmt.Sprintf("clustered_%s.html", timestamp)
	config := map[string]any{"Year": time.Now().Year(), "Period": "Clustered summary"}
	if err := exporter.ExportHTML(clustered, stats, htmlFile, clusterAuthor, config); err != nil {
		fmt.Printf("Failed to export HTML: %v\n", err)
		return
	}

	fmt.Printf("\nReports saved to %s/\n", clusterOutput)
	fmt.Printf("  -> %s (JSON)\n", jsonFile)
	fmt.Printf("  -> %s (HTML)\n", htmlFile)
}
