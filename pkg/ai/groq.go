package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GroqClient struct {
	APIKey string
}

func NewGroqClient(apiKey string) *GroqClient {
	return &GroqClient{APIKey: apiKey}
}

type GroqRequest struct {
	Model    string        `json:"model"`
	Messages []GroqMessage `json:"messages"`
}

type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (c *GroqClient) GenerateReport(ctx context.Context, model, systemPrompt, data string) (string, Usage, error) {
	url := "https://api.groq.com/openai/v1/chat/completions"

	var usage Usage

	reqBody := GroqRequest{
		Model: model,
		Messages: []GroqMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: data,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", usage, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", usage, err
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", usage, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", usage, fmt.Errorf("groq api error: %s - %s", resp.Status, string(body))
	}

	var groqResp GroqResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return "", usage, err
	}

	if len(groqResp.Choices) > 0 {
		return groqResp.Choices[0].Message.Content, groqResp.Usage, nil
	}

	return "", usage, fmt.Errorf("no response from groq")
}
