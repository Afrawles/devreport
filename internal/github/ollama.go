package github

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
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done       bool   `json:"done"`
	DoneReason string `json:"done_reason"`
}

// rephraseCommit takes a git commit message and rephrases it as a professional achievement.
func rephraseCommit(message string) string {
	if strings.TrimSpace(message) == "" {
		return message
	}

	client := &http.Client{Timeout: 30 * time.Second}

	prompt := "Rephrase the following git commit message as a concise, professional achievement bullet point.\n\n" +
		"STRICT RULES:\n" +
		"1. Use strong action verbs and focus on the accomplishment\n" +
		"2. PRESERVE all numerical values, version numbers, and identifiers EXACTLY as written\n" +
		"3. Only fix spelling errors and grammar mistakes\n" +
		"4. Do NOT change the core meaning or technical details\n" +
		"5. Keep it concise — one sentence max\n" +
		"6. Return only the rephrased text without bullet point symbols (•, -, *)\n\n" +
		"Commit message:\n" +
		message

	rephrased, err := callOllama(client, prompt)
	if err != nil {
		fmt.Printf("Ollama unavailable for commit rephrase: %v\n", err)
		return message
	}

	fmt.Printf("Rephrased commit: %s -> %s\n", firstLine(message), rephrased)
	return rephrased
}

// rephrasePR takes a PR title and body and rephrases it as a professional achievement.
func rephrasePR(title, body string) string {
	input := title
	if strings.TrimSpace(body) != "" {
		input = title + "\n\n" + body
	}

	if strings.TrimSpace(input) == "" {
		return input
	}

	client := &http.Client{Timeout: 30 * time.Second}

	prompt := "Rephrase the following pull request title and description as a concise, professional achievement bullet point.\n\n" +
		"STRICT RULES:\n" +
		"1. Use strong action verbs and focus on the accomplishment\n" +
		"2. PRESERVE all numerical values, version numbers, and identifiers EXACTLY as written\n" +
		"3. Only fix spelling errors and grammar mistakes\n" +
		"4. Do NOT change the core meaning or technical details\n" +
		"5. Keep it concise — one to two sentences max\n" +
		"6. Return only the rephrased text without bullet point symbols (•, -, *)\n\n" +
		"Pull request:\n" +
		input

	rephrased, err := callOllama(client, prompt)
	if err != nil {
		fmt.Printf("Ollama unavailable for PR rephrase: %v\n", err)
		return title
	}

	fmt.Printf("Rephrased PR: %s -> %s\n", title, rephrased)
	return rephrased
}

func callOllama(client *http.Client, prompt string) (string, error) {
	reqBody, err := json.Marshal(ollamaChatRequest{
		// TODO: make this configurable
		Model: "gemma4:e4b",
		Messages: []ollamaMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := client.Post("http://localhost:11434/api/chat", "application/json", bytes.NewBuffer(reqBody))
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

	rephrased := strings.TrimSpace(parsed.Message.Content)
	if rephrased == "" {
		return "", fmt.Errorf("ollama returned empty content")
	}

	return rephrased, nil
}
