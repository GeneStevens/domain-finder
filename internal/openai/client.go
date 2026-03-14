package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gene/domain-finder/internal/config"
)

const chatCompletionsURL = "https://api.openai.com/v1/chat/completions"

const systemPrompt = "You generate candidate domain stems. Return JSON only. Produce concise single-label stems only, no TLDs, no dots, no spaces, no numbering, no explanations, lowercase preferred."

// StemGenerator produces candidate stem batches.
type StemGenerator interface {
	GenerateBatch(ctx context.Context, prompt string, count int) ([]string, error)
}

// Client is a minimal OpenAI Chat Completions client for batch stem generation.
type Client struct {
	APIKey  string
	Model   string
	BaseURL string
	HTTP    *http.Client
}

// NewClient creates a configured client from resolved config.
func NewClient(cfg config.Config) (*Client, error) {
	if cfg.OpenAI.APIKey == "" {
		return nil, fmt.Errorf("missing OpenAI API key; set OPENAI_API_KEY or domainfinder.local.yaml")
	}
	return &Client{
		APIKey:  cfg.OpenAI.APIKey,
		Model:   cfg.OpenAI.Model,
		BaseURL: chatCompletionsURL,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// GenerateBatch requests one batch of candidate stems.
func (c *Client) GenerateBatch(ctx context.Context, prompt string, count int) ([]string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type request struct {
		Model          string         `json:"model"`
		Messages       []message      `json:"messages"`
		ResponseFormat responseFormat `json:"response_format"`
	}

	reqBody := request{
		Model: c.Model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Generate %d candidate stems as JSON for: %s", count, prompt)},
		},
		ResponseFormat: responseFormat{
			Type: "json_schema",
			JSONSchema: jsonSchema{
				Name:   "stem_batch",
				Strict: true,
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"stems": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
					},
					"required":             []string{"stems"},
					"additionalProperties": false,
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error %d: %s", resp.StatusCode, raw)
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("openai returned no choices")
	}

	var payload struct {
		Stems []string `json:"stems"`
	}
	if err := json.Unmarshal([]byte(parsed.Choices[0].Message.Content), &payload); err != nil {
		return nil, fmt.Errorf("decode stems payload: %w", err)
	}
	return payload.Stems, nil
}

type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type jsonSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
