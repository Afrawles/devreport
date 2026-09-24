package llm

import "strings"

type Config struct {
	Provider     string
	OllamaModel  string
	ClaudeAPIKey string
	ClaudeModel  string
}

func New(cfg Config) Provider {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "claude":
		return NewClaudeProvider(cfg.ClaudeAPIKey, cfg.ClaudeModel)
	default:
		return NewOllamaProvider(cfg.OllamaModel)
	}
}
