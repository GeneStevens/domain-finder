package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/genestevens/domain-finder/internal/config"
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
		return nil, fmt.Errorf("missing OpenAI API key; set OPENAI_API_KEY or domain-finder.local.yaml")
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
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, &GenerationError{Kind: ErrorTransient, Message: "openai request failed", Err: err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		kind := ErrorProtocol
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			kind = ErrorTransient
		}
		return nil, &GenerationError{
			Kind:       kind,
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(raw)),
		}
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, &GenerationError{Kind: ErrorProtocol, Message: "decode response", Err: err}
	}
	if len(parsed.Choices) == 0 {
		return nil, &GenerationError{Kind: ErrorProtocol, Message: "openai returned no choices"}
	}
	stems, err := extractStems(parsed.Choices[0].Message.Content)
	if err != nil {
		return nil, err
	}
	return stems, nil
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

func extractStems(content string) ([]string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, &GenerationError{Kind: ErrorQuality, Message: "empty stems payload"}
	}
	if stems, ok := parseJSONStemPayload(trimmed); ok {
		return stems, nil
	}
	if start := strings.Index(trimmed, "{"); start >= 0 {
		if end := strings.LastIndex(trimmed, "}"); end > start {
			if stems, ok := parseJSONStemPayload(trimmed[start : end+1]); ok {
				return stems, nil
			}
		}
	}
	stems := extractLooseStemLines(trimmed)
	if len(stems) == 0 {
		return nil, &GenerationError{Kind: ErrorQuality, Message: "no candidate stems found in model output"}
	}
	return stems, nil
}

func parseJSONStemPayload(value string) ([]string, bool) {
	var payload struct {
		Stems []string `json:"stems"`
	}
	if err := json.Unmarshal([]byte(value), &payload); err != nil || payload.Stems == nil {
		return nil, false
	}
	return payload.Stems, true
}

func extractLooseStemLines(value string) []string {
	normalized := strings.NewReplacer("\r\n", "\n", "\r", "\n", ";", "\n", ",", "\n").Replace(value)
	lines := strings.Split(normalized, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		candidate := strings.TrimSpace(line)
		candidate = strings.TrimLeft(candidate, "-*• \t")
		candidate = strings.TrimSpace(stripLeadingEnumeration(candidate))
		candidate = strings.Trim(candidate, "`\"'")
		if candidate == "" {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func stripLeadingEnumeration(value string) string {
	index := 0
	for index < len(value) && unicode.IsDigit(rune(value[index])) {
		index++
	}
	if index == 0 || index >= len(value) {
		return value
	}
	switch value[index] {
	case '.', ')', ':', '-':
		return strings.TrimSpace(value[index+1:])
	default:
		return value
	}
}
