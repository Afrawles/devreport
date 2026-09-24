package report

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Afrawles/devreport/internal/llm"
)

type Clusterer struct {
	LLM       llm.Provider
	BatchSize int
	MinGroup  int
}

func NewClusterer(p llm.Provider, batchSize, minGroup int) *Clusterer {
	if batchSize <= 0 {
		batchSize = 40
	}
	if minGroup <= 0 {
		minGroup = 5
	}
	return &Clusterer{LLM: p, BatchSize: batchSize, MinGroup: minGroup}
}

// ClusterBySource groups tasks by Source and asks the LLM to merge related or
// small tasks within each source into combined rows. Sources with fewer than
// MinGroup tasks, and any batch whose LLM response can't be parsed, are
// passed through unchanged rather than dropped.
func (c *Clusterer) ClusterBySource(tasks []Task) []Task {
	bySource := map[string][]Task{}
	var order []string
	for _, t := range tasks {
		if _, ok := bySource[t.Source]; !ok {
			order = append(order, t.Source)
		}
		bySource[t.Source] = append(bySource[t.Source], t)
	}

	var out []Task
	for _, source := range order {
		group := bySource[source]
		if len(group) < c.MinGroup {
			out = append(out, group...)
			continue
		}
		out = append(out, c.clusterGroup(group)...)
	}
	return out
}

func (c *Clusterer) clusterGroup(tasks []Task) []Task {
	var result []Task
	for start := 0; start < len(tasks); start += c.BatchSize {
		end := start + c.BatchSize
		if end > len(tasks) {
			end = len(tasks)
		}
		batch := tasks[start:end]
		clustered, err := c.clusterBatch(batch)
		if err != nil {
			fmt.Printf("Clustering failed for %s (batch %d-%d), keeping ungrouped: %v\n", tasks[0].Source, start, end, err)
			result = append(result, batch...)
			continue
		}
		result = append(result, clustered...)
	}
	return result
}

type clusterResponse struct {
	Clusters []struct {
		Title       string   `json:"title"`
		Achievement string   `json:"achievement"`
		MemberIDs   []string `json:"member_ids"`
	} `json:"clusters"`
}

func (c *Clusterer) clusterBatch(batch []Task) ([]Task, error) {
	byID := map[string]Task{}
	var sb strings.Builder
	sb.WriteString("Group the following pull requests/tasks into a small number of clusters of closely related work. ")
	sb.WriteString("Each cluster should combine tasks describing the same feature area or closely related fixes. ")
	sb.WriteString("Write ONE combined technical achievement (1-3 sentences) per cluster, in the same technical voice as the " +
		"input achievements — preserve function/model/field names and specifics, do not simplify into business language. ")
	sb.WriteString("A task with no close relative can be its own single-member cluster. Every input id must appear in exactly one cluster.\n\n")
	sb.WriteString("Respond with ONLY valid JSON, no markdown fences, no commentary, in this exact shape:\n")
	sb.WriteString(`{"clusters":[{"title":"short theme name","achievement":"combined technical sentence(s)","member_ids":["id1","id2"]}]}`)
	sb.WriteString("\n\nTasks:\n")
	for _, t := range batch {
		byID[t.ID] = t
		sb.WriteString(fmt.Sprintf("- id: %s | title: %s | achievement: %s\n", t.ID, t.Title, t.Achievements))
	}

	resp, err := c.LLM.Complete(sb.String())
	if err != nil {
		return nil, err
	}

	var parsed clusterResponse
	if err := json.Unmarshal([]byte(extractJSONObject(resp)), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse cluster response: %w", err)
	}

	seen := map[string]bool{}
	var out []Task
	for _, cl := range parsed.Clusters {
		var members []Task
		for _, id := range cl.MemberIDs {
			if t, ok := byID[id]; ok && !seen[id] {
				members = append(members, t)
				seen[id] = true
			}
		}
		if len(members) == 0 {
			continue
		}
		out = append(out, mergeCluster(cl.Title, cl.Achievement, members))
	}

	// Any task the LLM didn't place anywhere is kept standalone rather than dropped.
	for _, t := range batch {
		if !seen[t.ID] {
			out = append(out, t)
		}
	}

	return out, nil
}

func mergeCluster(title, achievement string, members []Task) Task {
	earliest := members[0].CreatedAt
	var latestCompleted *time.Time
	statusCounts := map[string]int{}
	var ids []string

	for _, m := range members {
		if m.CreatedAt.Before(earliest) {
			earliest = m.CreatedAt
		}
		if m.CompletedAt != nil && (latestCompleted == nil || m.CompletedAt.After(*latestCompleted)) {
			latestCompleted = m.CompletedAt
		}
		statusCounts[m.Status]++
		ids = append(ids, m.ID)
	}

	status := members[0].Status
	best := 0
	for s, n := range statusCounts {
		if n > best {
			best = n
			status = s
		}
	}

	if title == "" {
		title = members[0].Title
	}
	if achievement == "" {
		achievement = members[0].Achievements
	}

	return Task{
		ID:           "combined-" + ids[0],
		Title:        title,
		Description:  "PRs: " + strings.Join(ids, ", "),
		Status:       status,
		Source:       members[0].Source,
		Type:         members[0].Type,
		Assignee:     members[0].Assignee,
		CreatedAt:    earliest,
		UpdatedAt:    earliest,
		CompletedAt:  latestCompleted,
		Achievements: achievement,
	}
}

// extractJSONObject pulls the outermost {...} out of a model response that may
// contain markdown fences, stray prose, or reasoning text around the JSON.
func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end < start {
		return s
	}
	return s[start : end+1]
}
