package clickup

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

// rephraseBatch takes a slice of achievements and rephrases them using Ollama.
func rephraseBatch(achievements []string) []string {
	if len(achievements) == 0 {
		return achievements
	}

	client := &http.Client{Timeout: 60 * time.Second}

	prompt := "Rephrase each bullet point below to be concise and professional using verbs. " +
		"Keep the bullet point format (•). Return exactly one line per input, in the same order:\n\n" +
		strings.Join(achievements, "\n")

	reqBody, err := json.Marshal(ollamaChatRequest{
		Model: "gemma3",
		Messages: []ollamaMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	})
	if err != nil {
		fmt.Printf("Failed to marshal Ollama request, using original achievements: %v\n", err)
		return achievements
	}

	resp, err := client.Post("http://localhost:11434/api/chat", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		fmt.Printf("Ollama unavailable, using original achievements: %v\n", err)
		return achievements
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Ollama returned status %d, using original achievements\n", resp.StatusCode)
		return achievements
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read Ollama response, using original achievements: %v\n", err)
		return achievements
	}

	var parsed ollamaChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		fmt.Printf("Failed to decode Ollama response: %v\n", err)
		return achievements
	}

	content := strings.TrimSpace(parsed.Message.Content)
	if content == "" {
		fmt.Printf("Ollama returned empty content, using original achievements\n")
		return achievements
	}

	// if len(content) > 200 {
	// 	fmt.Printf("Ollama response preview: %s...\n", content[:200])
	// }

	rephrased := strings.Split(content, "\n")

	var cleanedRephrased []string
	for _, line := range rephrased {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanedRephrased = append(cleanedRephrased, trimmed)
		}
	}
	//
	// if len(cleanedRephrased) != len(achievements) {
	// 	fmt.Printf("Ollama returned %d lines, expected %d; using original achievements\n",
	// 		len(cleanedRephrased), len(achievements))
	// 	return achievements
	// }

	fmt.Printf("Successfully rephrased %d achievements using Ollama\n", len(cleanedRephrased))
	return cleanedRephrased
}
