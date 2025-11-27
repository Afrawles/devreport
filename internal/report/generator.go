package report

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type Generator struct {
	Sources []ActivitySource
}

func NewGenerator(sources ...ActivitySource) *Generator {
	return &Generator{Sources: sources}
}

// Generate fetches tasks from all sources and aggregates them
func (g *Generator) Generate(ctx context.Context, user string, start, end time.Time) ([]Task, error) {
	var all []Task
	errors := make(map[string]error)

	for _, src := range g.Sources {
		fmt.Printf("<<<< fetching tasks from >>>> %s...\n", src.Name())

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if err := src.HealthCheck(); err != nil {
			errors[src.Name()] = fmt.Errorf("health check failed: %w", err)
			fmt.Printf("%s is unavailable: %v\n", src.Name(), err)
			continue
		}

		tasks, err := src.FetchTasks(user, start, end)
		if err != nil {
			errors[src.Name()] = err
			fmt.Printf("Error fetching from %s: %v\n", src.Name(), err)
			continue
		}

		fmt.Printf("Fetched %d tasks from %s\n", len(tasks), src.Name())
		all = append(all, tasks...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	if len(all) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("failed to fetch from all sources: %v", errors)
	}

	return all, nil
}

// Statistics generates summary stats
func (g *Generator) Statistics(tasks []Task) map[string]any {
	stats := make(map[string]any)

	bySource := make(map[string]int)
	byStatus := make(map[string]int)
	byType := make(map[string]int)

	completed := 0
	for _, task := range tasks {
		bySource[task.Source]++
		byStatus[task.Status]++
		byType[task.Type]++
		if task.CompletedAt != nil {
			completed++
		}
	}

	stats["total"] = len(tasks)
	stats["completed"] = completed
	stats["by_source"] = bySource
	stats["by_status"] = byStatus
	stats["by_type"] = byType
	return stats
}
