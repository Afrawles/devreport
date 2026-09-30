package report

import (
	"fmt"
	"sort"
	"strings"
)

// NoteColumns holds the raw --challenges, --support-required, --support-from
// and --follow-up flag values.
//
// Each value is a comma-separated list of groups, one group per project (a
// ClickUp list or a GitHub repo); "|" separates bullets inside a group. A
// group can name its project ("Smart Parking=Delayed feedback|Unclear specs");
// unnamed groups are matched to projects by position (see ProjectOrder).
type NoteColumns struct {
	Challenges      string
	SupportRequired string
	SupportFrom     string
	FollowUp        string
}

func (n NoteColumns) empty() bool {
	return n.Challenges == "" && n.SupportRequired == "" && n.SupportFrom == "" && n.FollowUp == ""
}

// projectName is the project a task is shown under in the HTML report.
func projectName(t Task) string {
	if t.Source == "" {
		return "Uncategorized"
	}
	return t.Source
}

// ProjectOrder returns the projects that positional note groups map to, in
// order: first the explicit entries (ClickUp list IDs, then GitHub repo names,
// in the order they were passed), then any other project alphabetically — the
// order the HTML report shows them in. An explicit entry with no tasks in the
// period still takes its slot so the groups after it do not shift.
func ProjectOrder(tasks []Task, explicit []string) []string {
	byID := map[string]string{}
	byName := map[string]string{}
	for _, t := range tasks {
		name := projectName(t)
		if t.ProjectID != "" {
			byID[t.ProjectID] = name
		}
		byName[strings.ToLower(name)] = name
	}

	var order []string
	used := map[string]bool{}
	for _, e := range explicit {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if i := strings.LastIndex(e, "/"); i >= 0 { // owner/repo -> repo
			e = e[i+1:]
		}
		name := e
		if n, ok := byID[e]; ok {
			name = n
		} else if n, ok := byName[strings.ToLower(e)]; ok {
			name = n
		}
		if used[strings.ToLower(name)] {
			continue
		}
		used[strings.ToLower(name)] = true
		order = append(order, name)
	}

	var rest []string
	for lower, name := range byName {
		if !used[lower] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(order, rest...)
}

// ApplyProjectNotes writes the note columns onto the first task (in report
// order) of each project, instead of onto whichever task happens to be at the
// same index. It returns warnings for groups that could not be placed.
func ApplyProjectNotes(tasks []Task, order []string, notes NoteColumns) []string {
	if notes.empty() {
		return nil
	}

	first := map[string]int{}
	for i, t := range tasks {
		if _, ok := first[strings.ToLower(projectName(t))]; !ok {
			first[strings.ToLower(projectName(t))] = i
		}
	}
	lookup := map[string]string{}
	for _, name := range order {
		lookup[strings.ToLower(name)] = name
	}
	for _, t := range tasks {
		lookup[strings.ToLower(projectName(t))] = projectName(t)
		if t.ProjectID != "" {
			lookup[strings.ToLower(t.ProjectID)] = projectName(t)
		}
	}

	var warnings []string
	apply := func(flag, raw string, set func(t *Task, text string)) {
		pos := 0
		for _, group := range strings.Split(raw, ",") {
			group = strings.TrimSpace(group)
			if group == "" {
				pos++
				continue
			}

			project := ""
			text := group
			if key, rest, ok := strings.Cut(group, "="); ok {
				if name, found := lookup[strings.ToLower(strings.TrimSpace(key))]; found {
					project, text = name, rest
				}
			}
			if project == "" {
				if pos >= len(order) {
					warnings = append(warnings, fmt.Sprintf("--%s: %q has no project to go to (only %d projects)", flag, group, len(order)))
					continue
				}
				project = order[pos]
				pos++
			}

			idx, ok := first[strings.ToLower(project)]
			if !ok {
				warnings = append(warnings, fmt.Sprintf("--%s: no activities for %q in this period, skipped %q", flag, project, text))
				continue
			}
			set(&tasks[idx], formatBullets(text))
		}
	}

	apply("challenges", notes.Challenges, func(t *Task, s string) { t.Challenges = s })
	apply("support-required", notes.SupportRequired, func(t *Task, s string) { t.SupportRequired = s })
	apply("support-from", notes.SupportFrom, func(t *Task, s string) { t.SupportFrom = s })
	apply("follow-up", notes.FollowUp, func(t *Task, s string) { t.FollowUp = s })
	return warnings
}

// formatBullets turns "a|b" into "• a\n• b"; a single item is left as is.
func formatBullets(s string) string {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "|") {
		return s
	}
	var out []string
	for _, b := range strings.Split(s, "|") {
		if b = strings.TrimSpace(b); b != "" {
			out = append(out, "• "+b)
		}
	}
	return strings.Join(out, "\n")
}
