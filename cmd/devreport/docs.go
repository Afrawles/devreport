package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Afrawles/devreport/internal/gitlog"
	"github.com/Afrawles/devreport/internal/llm"
	"github.com/Afrawles/devreport/internal/report"
	"github.com/spf13/cobra"
)

var (
	docsRepos       []string
	docsAliases     []string
	docsStart       string
	docsEnd         string
	docsBranch      string
	docsGroup       string
	docsDevs        string
	docsAuthor      string
	docsDept        string
	docsPeriod      string
	docsYear        int
	docsTemplate    string
	docsOutput      string
	docsInput       string
	docsBatch       int
	docsMaxBullets  int
	docsOllamaModel string
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Word (.docx) report of work grouped by module, from local git repos",
	Long: `Reads commits (and the files they changed) from local git repositories,
groups them into feature modules per codebase (e.g. Backend, Web, Mobile) with
the developers who worked on each, and writes a Word report in the
"Individual Report" table layout. Fixes are placed in the module they touch.

Grouping modes (--group):
  llm   an LLM names the modules and writes the achievement bullets (default);
        commits it cannot place fall back to path grouping
  path  no LLM: modules come from the folders each commit changed and the
        bullets are the cleaned-up commit messages

Challenges are filled in only where the commits show a real problem (a crash,
a revert, a rollback, a workaround...); otherwise the cell stays empty.

A modules JSON is saved next to the .docx. Edit it (rename or merge modules,
correct challenges, fill in support and follow up) and re-render with --input.`,
	Example: `  devreport docs \
    --repo "Backend=~/work/iras/pidmis-backend" \
    --repo "Web=~/work/iras/pidmis-frontend" \
    --repo "Mobile=~/work/iras/pidmis-mobile" \
    --start 2026-07-01 --end 2026-09-30 \
    --author "Moses Odeke" --dept "Software Development" \
    --alias "izaiah-m=Izaiah Mukisa" --alias "ConradKash=Conrad Kakuru" \
    --template "Individual Report Template.docx" \
    --llm-provider claude

  # re-render after editing the JSON
  devreport docs --input reports/modules_20260930_120000.json --template "Individual Report Template.docx"`,
	Run: generateDocs,
}

func init() {
	rootCmd.AddCommand(docsCmd)

	f := docsCmd.Flags()
	f.StringArrayVar(&docsRepos, "repo", nil, `Repository as "Label=path" (repeatable); label defaults to the folder name`)
	f.StringVar(&docsStart, "start", "", "Start date YYYY-MM-DD (default: first day of this month)")
	f.StringVar(&docsEnd, "end", "", "End date YYYY-MM-DD (default: today)")
	f.StringVar(&docsBranch, "branch", "", "Branch to read (default: all branches)")
	f.StringVar(&docsGroup, "group", report.GroupLLM, "Grouping mode: llm or path")
	f.StringVar(&docsDevs, "dev", "", "Only include commits by these developers (comma-separated, matches name or email)")
	f.StringArrayVar(&docsAliases, "alias", nil, `Map a git author name/handle/email to a display name, "izaiah-m=Izaiah Mukisa" (repeatable)`)
	f.StringVar(&docsAuthor, "author", "", "Submitted by")
	f.StringVar(&docsDept, "dept", "", "Department (left as a highlighted placeholder if empty)")
	f.StringVar(&docsPeriod, "period", "", `Period label (default from dates, e.g. "July – September 2026")`)
	f.IntVar(&docsYear, "year", 0, "Year shown in the report header (default: end date year)")
	f.StringVar(&docsTemplate, "template", "", "Report template .docx; its first table is replaced with the generated one")
	f.StringVar(&docsOutput, "output", "reports", "Output directory")
	f.StringVar(&docsInput, "input", "", "Render a previously saved modules JSON instead of reading git")
	f.IntVar(&docsBatch, "batch-size", 60, "Max commits sent to the LLM per call")
	f.IntVar(&docsMaxBullets, "max-bullets", 6, "Max achievement bullets per module")

	f.StringVar(&llmProvider, "llm-provider", "ollama", "LLM backend for grouping: ollama or claude")
	f.StringVar(&claudeAPIKey, "claude-api-key", "", "Anthropic API key (or ANTHROPIC_API_KEY env var)")
	f.StringVar(&claudeModel, "claude-model", "", "Anthropic model id (default: claude-sonnet-5)")
	f.StringVar(&docsOllamaModel, "ollama-model", "gemma4:e4b", "Ollama model id to use for grouping")
}

func generateDocs(cmd *cobra.Command, args []string) {
	if err := os.MkdirAll(docsOutput, 0755); err != nil {
		fmt.Printf("Failed to create output directory: %v\n", err)
		return
	}
	stamp := time.Now().Format("20060102_150405")
	exporter := report.DocxExporter{TemplatePath: expandHome(docsTemplate)}

	if docsInput != "" {
		r, err := report.LoadModuleReport(docsInput)
		if err != nil {
			fmt.Printf("Failed to load %s: %v\n", docsInput, err)
			return
		}
		applyHeaderFlags(cmd, &r)
		docxFile := filepath.Join(docsOutput, fmt.Sprintf("report_%s.docx", stamp))
		if err := exporter.Export(r, docxFile); err != nil {
			fmt.Printf("Failed to export docx: %v\n", err)
			return
		}
		fmt.Printf("Report saved to %s\n", docxFile)
		return
	}

	if len(docsRepos) == 0 {
		fmt.Println(`At least one --repo is required, e.g. --repo "Backend=~/work/app-backend"`)
		return
	}

	start, end, err := docsDateRange()
	if err != nil {
		fmt.Println(err)
		return
	}
	if docsGroup != report.GroupLLM && docsGroup != report.GroupPath {
		fmt.Printf("Unknown --group %q (use llm or path)\n", docsGroup)
		return
	}

	type repoCommits struct {
		label, name string
		commits     []gitlog.Commit
	}
	var sections []repoCommits
	var all []gitlog.Commit
	sectionIndex := map[string]int{}
	seenHash := map[string]bool{}

	for _, spec := range docsRepos {
		label, path := parseRepoSpec(spec)
		fmt.Printf("Reading %s (%s)...\n", label, path)
		commits, err := gitlog.Read(path, gitlog.Options{Branch: docsBranch, Since: start, Until: end, IncludeMerges: true})
		if err != nil {
			fmt.Printf("  %v\n", err)
			continue
		}

		idx, ok := sectionIndex[strings.ToLower(label)]
		if !ok {
			idx = len(sections)
			sectionIndex[strings.ToLower(label)] = idx
			sections = append(sections, repoCommits{label: label, name: filepath.Base(path)})
		}
		added := 0
		for _, c := range commits {
			if seenHash[c.Hash] { // same repo cloned twice, or listed twice
				continue
			}
			seenHash[c.Hash] = true
			all = append(all, c) // merges too: they help match handles to names
			if c.Merge {
				continue
			}
			sections[idx].commits = append(sections[idx].commits, c)
			added++
		}
		fmt.Printf("  %d commits\n", added)
	}

	aliases := map[string]string{}
	for _, a := range docsAliases {
		if k, v, ok := strings.Cut(a, "="); ok {
			aliases[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	ids := report.NewIdentities(all, aliases)

	if docsDevs != "" {
		wanted := parseList(docsDevs)
		keep := func(c gitlog.Commit) bool {
			name := strings.ToLower(ids.Name(c))
			for _, w := range wanted {
				w = strings.ToLower(w)
				if strings.Contains(name, w) || strings.Contains(c.AuthorEmail, w) || strings.Contains(strings.ToLower(c.AuthorName), w) {
					return true
				}
			}
			return false
		}
		for i := range sections {
			var kept []gitlog.Commit
			for _, c := range sections[i].commits {
				if keep(c) {
					kept = append(kept, c)
				}
			}
			sections[i].commits = kept
		}
	}

	grouper := &report.ModuleGrouper{
		Mode:       docsGroup,
		BatchSize:  docsBatch,
		MaxBullets: docsMaxBullets,
		Names:      ids,
	}
	if docsGroup == report.GroupLLM {
		cfg := resolveLLMConfig(cmd)
		cfg.OllamaModel = docsOllamaModel
		provider := llm.New(cfg)
		if c, ok := provider.(*llm.ClaudeProvider); ok {
			c.MaxTokens = 8192
			c.Client = &http.Client{Timeout: 3 * time.Minute}
		}
		grouper.LLM = provider
		fmt.Printf("Grouping with %s\n", provider.Name())
	}

	r := report.ModuleReport{
		Dept:        docsDept,
		SubmittedBy: docsAuthor,
		Period:      docsPeriod,
		Year:        docsYear,
	}
	if r.Period == "" {
		r.Period = report.PeriodLabel(start, end)
	}
	if r.Year == 0 {
		r.Year = end.Year()
	}

	total := 0
	for _, s := range sections {
		if len(s.commits) == 0 {
			continue
		}
		fmt.Printf("Grouping %d %s commits...\n", len(s.commits), s.label)
		modules := grouper.Group(s.label, s.name, s.commits)
		r.Sections = append(r.Sections, report.Section{
			Title:   fmt.Sprintf("%s (%s)", strings.ToUpper(s.label), s.name),
			Modules: modules,
		})
		total += len(s.commits)
	}
	if total == 0 {
		fmt.Println("No commits found for this period")
		return
	}

	jsonFile := filepath.Join(docsOutput, fmt.Sprintf("modules_%s.json", stamp))
	if err := r.SaveJSON(jsonFile); err != nil {
		fmt.Printf("Failed to save JSON: %v\n", err)
	}
	docxFile := filepath.Join(docsOutput, fmt.Sprintf("report_%s.docx", stamp))
	if err := exporter.Export(r, docxFile); err != nil {
		fmt.Printf("Failed to export docx: %v\n", err)
		return
	}

	fmt.Printf("\nReports saved to %s/\n", docsOutput)
	fmt.Printf("  -> %s (Word)\n", filepath.Base(docxFile))
	fmt.Printf("  -> %s (edit and re-render with --input)\n", filepath.Base(jsonFile))

	printDevelopers(sections2commits(r, all), ids)
}

// sections2commits returns the commits that ended up in the report.
func sections2commits(r report.ModuleReport, all []gitlog.Commit) []gitlog.Commit {
	used := map[string]bool{}
	for _, s := range r.Sections {
		for _, m := range s.Modules {
			for _, c := range m.Commits {
				used[c] = true
			}
		}
	}
	var out []gitlog.Commit
	for _, c := range all {
		if used[c.Short] {
			out = append(out, c)
		}
	}
	return out
}

// printDevelopers lists who was credited and under which git identities, so
// handles that should be merged can be fixed with --alias.
func printDevelopers(commits []gitlog.Commit, ids *report.Identities) {
	counts := map[string]int{}
	raw := map[string]map[string]bool{}
	for _, c := range commits {
		n := ids.Name(c)
		counts[n]++
		if raw[n] == nil {
			raw[n] = map[string]bool{}
		}
		raw[n][c.AuthorName+" <"+c.AuthorEmail+">"] = true
	}
	var names []string
	for n := range counts {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return counts[names[i]] > counts[names[j]] })

	fmt.Println("\nDevelopers (use --alias to merge handles):")
	for _, n := range names {
		var idents []string
		for k := range raw[n] {
			idents = append(idents, k)
		}
		sort.Strings(idents)
		fmt.Printf("  %-28s %4d commits  %s\n", n, counts[n], strings.Join(idents, ", "))
	}
}

func applyHeaderFlags(cmd *cobra.Command, r *report.ModuleReport) {
	if cmd.Flags().Changed("author") {
		r.SubmittedBy = docsAuthor
	}
	if cmd.Flags().Changed("dept") {
		r.Dept = docsDept
	}
	if cmd.Flags().Changed("period") {
		r.Period = docsPeriod
	}
	if cmd.Flags().Changed("year") {
		r.Year = docsYear
	}
	if r.Year == 0 {
		r.Year = time.Now().Year()
	}
}

func docsDateRange() (time.Time, time.Time, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	end := now
	var err error
	if docsStart != "" {
		if start, err = time.ParseInLocation("2006-01-02", docsStart, time.Local); err != nil {
			return start, end, fmt.Errorf("invalid --start: %v", err)
		}
	}
	if docsEnd != "" {
		if end, err = time.ParseInLocation("2006-01-02", docsEnd, time.Local); err != nil {
			return start, end, fmt.Errorf("invalid --end: %v", err)
		}
	}
	if end.Before(start) {
		return start, end, fmt.Errorf("--end is before --start")
	}
	return start, end, nil
}

func parseRepoSpec(spec string) (label, path string) {
	if l, p, ok := strings.Cut(spec, "="); ok {
		label, path = strings.TrimSpace(l), strings.TrimSpace(p)
	} else {
		path = strings.TrimSpace(spec)
	}
	path = expandHome(path)
	if label == "" {
		label = filepath.Base(path)
	}
	return label, path
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func parseList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
