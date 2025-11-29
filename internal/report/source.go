package report

import "time"

type Task struct {
	ID              string
	Title           string
	Description     string
	Status          string
	URL             string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
	Source          string
	Type            string
	Labels          []string
	Assignee        string
	Achievements    string
	Challenges      string
	SupportRequired string
	SupportFrom     string
	FollowUp        string
	AttachmentURL   string
}

type ActivitySource interface {
	Name() string
	FetchTasks(user string, start, end time.Time) ([]Task, error)
	HealthCheck() error
}
