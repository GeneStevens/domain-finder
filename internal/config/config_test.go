package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPrecedence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, baseConfigName), []byte("openai:\n  model: base-model\npostgres:\n  dsn: postgres://base\ngenerate:\n  count: 5\n  batch_size: 2\n  quality_profile: industrial\n  max_length: 8\n  suffix: ix\n  avoid_substrings: dev,cloud\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, localConfigName), []byte("openai:\n  api_key: local-key\n  model: local-model\ngenerate:\n  count: 7\n  max_attempts: 4\n  retry_count: 1\n  max_syllables: 2\n  prefix: neo\n  style: developer tool\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		"OPENAI_API_KEY":                         "env-key",
		"DOMAINFINDER_OPENAI_MODEL":              "env-model",
		"DOMAINFINDER_GENERATE_BATCH_SIZE":       "4",
		"DOMAINFINDER_GENERATE_RETRY_COUNT":      "5",
		"DOMAINFINDER_GENERATE_QUALITY_PROFILE":  "off",
		"DOMAINFINDER_GENERATE_SUFFIX":           "io",
		"DOMAINFINDER_GENERATE_AVOID_SUBSTRINGS": "stack,forge",
	}
	cfg, err := Load(dir, func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}, CLIOverrides{
		OpenAIModel:             "cli-model",
		GenerateCount:           9,
		GenerateBatchSize:       6,
		GenerateQualityProfile:  "industrial",
		GenerateMaxLength:       12,
		GenerateMaxSyllables:    3,
		GeneratePrefix:          "dev",
		GenerateStyle:           "invented SaaS",
		GenerateAvoidSubstrings: "grid,flow,stack",
		PostgresDSN:             "postgres://cli",
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
	if cfg.Generate.MaxAttemptsPerBatch != 4 {
		t.Fatalf("Generate.MaxAttemptsPerBatch = %d, want 4", cfg.Generate.MaxAttemptsPerBatch)
	}
	if cfg.Generate.RetryCount != 5 {
		t.Fatalf("Generate.RetryCount = %d, want 5", cfg.Generate.RetryCount)
	}
	if cfg.Generate.QualityProfile != "industrial" {
		t.Fatalf("Generate.QualityProfile = %q, want industrial", cfg.Generate.QualityProfile)
	}
	if cfg.Generate.MaxLength != 12 {
		t.Fatalf("Generate.MaxLength = %d, want 12", cfg.Generate.MaxLength)
	}
	if cfg.Generate.MaxSyllables != 3 {
		t.Fatalf("Generate.MaxSyllables = %d, want 3", cfg.Generate.MaxSyllables)
	}
	if cfg.Generate.Prefix != "dev" {
		t.Fatalf("Generate.Prefix = %q, want dev", cfg.Generate.Prefix)
	}
	if cfg.Generate.Suffix != "io" {
		t.Fatalf("Generate.Suffix = %q, want io", cfg.Generate.Suffix)
	}
	if cfg.Generate.Style != "invented SaaS" {
		t.Fatalf("Generate.Style = %q, want invented SaaS", cfg.Generate.Style)
	}
	if got := strings.Join(cfg.Generate.AvoidSubstrings, ","); got != "grid,flow,stack" {
		t.Fatalf("Generate.AvoidSubstrings = %q, want grid,flow,stack", got)
	}
	if cfg.Postgres.DSN != "postgres://cli" {
		t.Fatalf("Postgres.DSN = %q, want %q", cfg.Postgres.DSN, "postgres://cli")
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
