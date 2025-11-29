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

// rephraseTask takes a ClickUp task description and rephrases it as a professional achievement.
func rephraseTask(taskDescription string) string {
	if strings.TrimSpace(taskDescription) == "" {
		return taskDescription
	}

	client := &http.Client{Timeout: 30 * time.Second}
	
	prompt := "Rephrase the following task description as a concise, professional achievement bullet point.\n\n" +
		"STRICT RULES:\n" +
		"1. Use strong action verbs and focus on the accomplishment\n" +
		"2. For currency: Add 'UGX' prefix to numbers that represent money (e.g., '5000' becomes 'UGX 5000')\n" +
		"3. PRESERVE all numerical values EXACTLY as written - do not modify, round, or change any numbers\n" +
		"4. Only fix spelling errors and grammar mistakes\n" +
		"5. Do NOT change the core meaning or description of the task\n" +
		"6. Return only the rephrased text without bullet point symbols (•, -, *)\n\n" +
		"Original description:\n" +
		taskDescription

	reqBody, err := json.Marshal(ollamaChatRequest{
		Model: "gemma3",
		Messages: []ollamaMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	})
	if err != nil {
		fmt.Printf("Failed to marshal Ollama request: %v\n", err)
		return taskDescription
	}

	resp, err := client.Post("http://localhost:11434/api/chat", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		fmt.Printf("Ollama unavailable: %v\n", err)
		return taskDescription
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Ollama returned status %d\n", resp.StatusCode)
		return taskDescription
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read Ollama response: %v\n", err)
		return taskDescription
	}

	var parsed ollamaChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		fmt.Printf("Failed to decode Ollama response: %v\n", err)
		return taskDescription
	}

	rephrased := strings.TrimSpace(parsed.Message.Content)
	if rephrased == "" {
		fmt.Printf("Ollama returned empty content\n")
		return taskDescription
	}

	fmt.Printf("Successfully rephrased task: %s -> %s\n", taskDescription, rephrased)
	return rephrased
}
