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

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type OllamaProvider struct {
	Model   string
	BaseURL string
	Client  *http.Client
}

func NewOllamaProvider(model string) *OllamaProvider {
	return &OllamaProvider{
		Model:   model,
		BaseURL: "http://localhost:11434",
		Client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OllamaProvider) Name() string {
	return "ollama"
}

func (o *OllamaProvider) Complete(prompt string) (string, error) {
	reqBody, err := json.Marshal(ollamaChatRequest{
		Model: o.Model,
		Messages: []ollamaMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := o.Client.Post(o.BaseURL+"/api/chat", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("ollama unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var parsed ollamaChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	content := strings.TrimSpace(parsed.Message.Content)
	if content == "" {
		return "", fmt.Errorf("ollama returned empty content")
	}

	return content, nil
}
