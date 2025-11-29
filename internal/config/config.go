package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
    GitHub  GitHubConfig
    ClickUp ClickUpConfig
    Output  OutputConfig
	Author string
}

type GitHubConfig struct {
    Token    string
    Username string
}

type ClickUpConfig struct {
    APIKey      string
    AssigneeIDs []string
	ListID string
}

type OutputConfig struct {
    Directory string
    Format    []string // json, markdown, html, pdf
}

func LoadFromEnv() (*Config, error) {
    cfg := &Config{
        ClickUp: ClickUpConfig{
            APIKey: os.Getenv("CLICKUP_API_KEY"),
        },
        Output: OutputConfig{
            Directory: getEnvOrDefault("OUTPUT_DIR", "reports"),
            Format:    strings.Split(getEnvOrDefault("OUTPUT_FORMAT", "json,markdown"), ","),
        },
    }

    if assigneeIDsStr := os.Getenv("CLICKUP_ASSIGNEE_IDS"); assigneeIDsStr != "" {
        ids := strings.Split(assigneeIDsStr, ",")
        for i := range ids {
            ids[i] = strings.TrimSpace(ids[i])
        }
        cfg.ClickUp.AssigneeIDs = ids
    }

    return cfg, nil
}

func (c *Config) Validate() error {
	hasSource := c.GitHub.Token != ""

    if c.GitHub.Token != "" {
        hasSource = true
    }

    if c.ClickUp.APIKey != "" {
        if len(c.ClickUp.AssigneeIDs) == 0 {
            return fmt.Errorf("CLICKUP_API_KEY provided but CLICKUP_ASSIGNEE_IDS missing")
        }
        hasSource = true
    }

    if !hasSource {
        return fmt.Errorf("no data sources configured (set GITHUB_TOKEN or CLICKUP_API_KEY)")
    }

    return nil
}

func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
