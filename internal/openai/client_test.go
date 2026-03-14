package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
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
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"stems\":[\"brandfoo\",\"noviq\"]}"}}]}`)),
			}, nil
		})},
	}
	got, err := client.GenerateBatch(context.Background(), "developer tools", 2)
	if err != nil {
		t.Fatalf("GenerateBatch() error = %v", err)
	}
	if len(got) != 2 || got[0] != "brandfoo" || got[1] != "noviq" {
		t.Fatalf("GenerateBatch() = %#v, want [brandfoo noviq]", got)
	}
}
