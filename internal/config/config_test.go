package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrecedence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, baseConfigName), []byte("openai:\n  model: base-model\ngenerate:\n  count: 5\n  batch_size: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, localConfigName), []byte("openai:\n  api_key: local-key\n  model: local-model\ngenerate:\n  count: 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		"OPENAI_API_KEY":                   "env-key",
		"DOMAINFINDER_OPENAI_MODEL":        "env-model",
		"DOMAINFINDER_GENERATE_BATCH_SIZE": "4",
	}
	cfg, err := Load(dir, func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}, CLIOverrides{
		OpenAIModel:       "cli-model",
		GenerateCount:     9,
		GenerateBatchSize: 6,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.OpenAI.APIKey != "env-key" {
		t.Fatalf("APIKey = %q, want %q", cfg.OpenAI.APIKey, "env-key")
	}
	if cfg.OpenAI.Model != "cli-model" {
		t.Fatalf("Model = %q, want %q", cfg.OpenAI.Model, "cli-model")
	}
	if cfg.Generate.Count != 9 {
		t.Fatalf("Generate.Count = %d, want 9", cfg.Generate.Count)
	}
	if cfg.Generate.BatchSize != 6 {
		t.Fatalf("Generate.BatchSize = %d, want 6", cfg.Generate.BatchSize)
	}
}

func TestLoadRejectsAPIKeyInBaseConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, baseConfigName), []byte("openai:\n  api_key: bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(dir, func(string) (string, bool) { return "", false }, CLIOverrides{}); err == nil {
		t.Fatal("Load() error = nil, want base config api_key rejection")
	}
}
