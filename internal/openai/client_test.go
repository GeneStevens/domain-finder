package openai

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/genestevens/domain-finder/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestGenerateBatch(t *testing.T) {
	client := &Client{
		APIKey:  "test-key",
		Model:   "gpt-4o-mini",
		BaseURL: "https://example.invalid/v1/chat/completions",
		Builder: PromptBuilder{},
		Generate: config.GenerateConfig{
			QualityProfile:  "industrial",
			Style:           "developer tool",
			MaxLength:       12,
			MaxSyllables:    3,
			Prefix:          "dev",
			Suffix:          "io",
			AvoidSubstrings: []string{"stack", "cloud"},
			AvoidPrefixes:   []string{"neo"},
			AvoidSuffixes:   []string{"ia", "ora"},
		},
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("Authorization = %q, want %q", got, "Bearer test-key")
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if !strings.Contains(string(body), `"json_schema"`) {
				t.Fatalf("request body = %s, want structured output request", string(body))
			}
			for _, fragment := range []string{"developer tool", "Quality profile: industrial", "infrastructure-like stems", "no more than 12 letters", "no more than 3 syllables", "start with `dev`", "end with `io`", "`stack`", "`cloud`", "`neo`", "`ia`"} {
				if !strings.Contains(string(body), fragment) {
					t.Fatalf("request body missing %q:\n%s", fragment, string(body))
				}
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"stems\":[\"brandfoo\",\"noviq\"]}"}}],"usage":{"prompt_tokens":120,"completion_tokens":18,"prompt_tokens_details":{"cached_tokens":40}}}`)),
			}, nil
		})},
	}
	got, err := client.GenerateBatch(context.Background(), "developer tools", 2)
	if err != nil {
		t.Fatalf("GenerateBatch() error = %v", err)
	}
	if len(got.Stems) != 2 || got.Stems[0] != "brandfoo" || got.Stems[1] != "noviq" {
		t.Fatalf("GenerateBatch() stems = %#v, want [brandfoo noviq]", got.Stems)
	}
	if got.Usage == nil {
		t.Fatal("GenerateBatch() usage = nil, want usage data")
	}
	if got.Usage.InputTokens != 120 || got.Usage.OutputTokens != 18 || got.Usage.CachedInputTokens != 40 {
		t.Fatalf("GenerateBatch() usage = %#v, want prompt 120 completion 18 cached 40", got.Usage)
	}
	estimate := EstimateUsage(client.Model, *got.Usage)
	if !estimate.PricingAvailable {
		t.Fatal("EstimateUsage() pricing unavailable, want known pricing")
	}
	if math.Abs(estimate.CostUSD-0.0000258) > 1e-10 {
		t.Fatalf("EstimateUsage() cost = %f, want 0.0000258", estimate.CostUSD)
	}
}

func TestGenerateBatchInvalidTopLevelJSON(t *testing.T) {
	client := &Client{
		APIKey:  "test-key",
		Model:   "gpt-4o-mini",
		BaseURL: "https://example.invalid/v1/chat/completions",
		Builder: PromptBuilder{},
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`not json at all`)),
			}, nil
		})},
	}

	_, err := client.GenerateBatch(context.Background(), "developer tools", 2)
	if err == nil {
		t.Fatal("GenerateBatch() error = nil, want protocol error")
	}
	var genErr *GenerationError
	if !errors.As(err, &genErr) || genErr.Kind != ErrorProtocol {
		t.Fatalf("GenerateBatch() error = %v, want protocol error", err)
	}
}

func TestGenerateBatchSalvagesNoisyOutput(t *testing.T) {
	client := &Client{
		APIKey:  "test-key",
		Model:   "gpt-4o-mini",
		BaseURL: "https://example.invalid/v1/chat/completions",
		Builder: PromptBuilder{},
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"Here are ideas:\n1. brandfoo\n2. invalid stem\n3. noviq.com\n4. trynex"}}]}`)),
			}, nil
		})},
	}

	got, err := client.GenerateBatch(context.Background(), "developer tools", 4)
	if err != nil {
		t.Fatalf("GenerateBatch() error = %v", err)
	}
	if len(got.Stems) != 5 || got.Stems[0] != "Here are ideas:" || got.Stems[1] != "brandfoo" || got.Stems[4] != "trynex" {
		t.Fatalf("GenerateBatch() stems = %#v, want salvaged loose lines", got.Stems)
	}
	if got.Usage != nil {
		t.Fatalf("GenerateBatch() usage = %#v, want nil when response usage is absent", got.Usage)
	}
}
