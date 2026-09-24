package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultClaudeModel = "claude-sonnet-5"

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type claudeResponse struct {
	Content []claudeContentBlock `json:"content"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type ClaudeProvider struct {
	APIKey  string
	Model   string
	BaseURL string
	Client  *http.Client
}

func NewClaudeProvider(apiKey, model string) *ClaudeProvider {
	if model == "" {
		model = defaultClaudeModel
	}
	return &ClaudeProvider{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: "https://api.anthropic.com",
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *ClaudeProvider) Name() string {
	return "claude"
}

func (c *ClaudeProvider) Complete(prompt string) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("claude api key not configured")
	}

	reqBody, err := json.Marshal(claudeRequest{
		Model:     c.Model,
		MaxTokens: 1024,
		Messages: []claudeMessage{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/v1/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude unavailable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var parsed claudeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return "", fmt.Errorf("claude returned status %d: %s", resp.StatusCode, parsed.Error.Message)
		}
		return "", fmt.Errorf("claude returned status %d", resp.StatusCode)
	}

	for _, block := range parsed.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			return strings.TrimSpace(block.Text), nil
		}
	}

	return "", fmt.Errorf("claude returned empty content")
}
