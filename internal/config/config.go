package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	baseConfigName  = "domain-finder.yaml"
	localConfigName = "domain-finder.local.yaml"
)

// Config is the resolved runtime configuration.
type Config struct {
	OpenAI   OpenAIConfig
	Generate GenerateConfig
	Postgres PostgresConfig
}

type OpenAIConfig struct {
	APIKey string
	Model  string
}

type GenerateConfig struct {
	Count               int
	BatchSize           int
	MaxAttemptsPerBatch int
	RetryCount          int
	MaxLength           int
	MaxSyllables        int
	Suffix              string
	Prefix              string
	Style               string
}

type PostgresConfig struct {
	DSN string
}

// CLIOverrides are the CLI-provided config overrides.
type CLIOverrides struct {
	OpenAIModel          string
	GenerateCount        int
	GenerateBatchSize    int
	GenerateMaxLength    int
	GenerateMaxSyllables int
	GenerateSuffix       string
	GeneratePrefix       string
	GenerateStyle        string
	PostgresDSN          string
}

type fileConfig struct {
	OpenAIAPIKey         string
	OpenAIModel          string
	GenerateCount        int
	GenerateBatchSize    int
	GenerateMaxAttempts  int
	GenerateRetryCount   int
	GenerateMaxLength    int
	GenerateMaxSyllables int
	GenerateSuffix       string
	GeneratePrefix       string
	GenerateStyle        string
	PostgresDSN          string
}

// Load resolves configuration using precedence:
// CLI > environment > local YAML > base YAML > built-in defaults.
func Load(dir string, lookupEnv func(string) (string, bool), cli CLIOverrides) (Config, error) {
	cfg := Config{
		OpenAI: OpenAIConfig{
			Model: "gpt-4o-mini",
		},
		Generate: GenerateConfig{
			Count:               20,
			BatchSize:           10,
			MaxAttemptsPerBatch: 3,
			RetryCount:          2,
		},
	}

	base, err := loadFile(filepath.Join(dir, baseConfigName))
	if err != nil {
		return Config{}, err
	}
	if base.OpenAIAPIKey != "" {
		return Config{}, fmt.Errorf("%s must not contain openai.api_key; use %s or OPENAI_API_KEY", baseConfigName, localConfigName)
	}
	applyFileConfig(&cfg, base)

	local, err := loadFile(filepath.Join(dir, localConfigName))
	if err != nil {
		return Config{}, err
	}
	applyFileConfig(&cfg, local)

	if value, ok := lookupEnv("OPENAI_API_KEY"); ok && value != "" {
		cfg.OpenAI.APIKey = value
	}
	if value, ok := lookupEnv("PG_DSN"); ok && value != "" {
		cfg.Postgres.DSN = value
	}
	if value, ok := lookupEnv("DOMAINFINDER_OPENAI_MODEL"); ok && value != "" {
		cfg.OpenAI.Model = value
	}
	if value, ok := lookupEnv("DOMAINFINDER_GENERATE_COUNT"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse DOMAINFINDER_GENERATE_COUNT: %w", err)
		}
		cfg.Generate.Count = parsed
	}
	if value, ok := lookupEnv("DOMAINFINDER_GENERATE_BATCH_SIZE"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse DOMAINFINDER_GENERATE_BATCH_SIZE: %w", err)
		}
		cfg.Generate.BatchSize = parsed
	}
	if value, ok := lookupEnv("DOMAINFINDER_GENERATE_MAX_ATTEMPTS"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse DOMAINFINDER_GENERATE_MAX_ATTEMPTS: %w", err)
		}
		cfg.Generate.MaxAttemptsPerBatch = parsed
	}
	if value, ok := lookupEnv("DOMAINFINDER_GENERATE_RETRY_COUNT"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse DOMAINFINDER_GENERATE_RETRY_COUNT: %w", err)
		}
		cfg.Generate.RetryCount = parsed
	}
	if value, ok := lookupEnv("DOMAINFINDER_GENERATE_MAX_LENGTH"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse DOMAINFINDER_GENERATE_MAX_LENGTH: %w", err)
		}
		cfg.Generate.MaxLength = parsed
	}
	if value, ok := lookupEnv("DOMAINFINDER_GENERATE_MAX_SYLLABLES"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse DOMAINFINDER_GENERATE_MAX_SYLLABLES: %w", err)
		}
		cfg.Generate.MaxSyllables = parsed
	}
	if value, ok := lookupEnv("DOMAINFINDER_GENERATE_SUFFIX"); ok && value != "" {
		cfg.Generate.Suffix = value
	}
	if value, ok := lookupEnv("DOMAINFINDER_GENERATE_PREFIX"); ok && value != "" {
		cfg.Generate.Prefix = value
	}
	if value, ok := lookupEnv("DOMAINFINDER_GENERATE_STYLE"); ok && value != "" {
		cfg.Generate.Style = value
	}

	if cli.OpenAIModel != "" {
		cfg.OpenAI.Model = cli.OpenAIModel
	}
	if cli.GenerateCount > 0 {
		cfg.Generate.Count = cli.GenerateCount
	}
	if cli.GenerateBatchSize > 0 {
		cfg.Generate.BatchSize = cli.GenerateBatchSize
	}
	if cli.GenerateMaxLength > 0 {
		cfg.Generate.MaxLength = cli.GenerateMaxLength
	}
	if cli.GenerateMaxSyllables > 0 {
		cfg.Generate.MaxSyllables = cli.GenerateMaxSyllables
	}
	if cli.GenerateSuffix != "" {
		cfg.Generate.Suffix = cli.GenerateSuffix
	}
	if cli.GeneratePrefix != "" {
		cfg.Generate.Prefix = cli.GeneratePrefix
	}
	if cli.GenerateStyle != "" {
		cfg.Generate.Style = cli.GenerateStyle
	}
	if cli.PostgresDSN != "" {
		cfg.Postgres.DSN = cli.PostgresDSN
	}

	return cfg, nil
}

func applyFileConfig(cfg *Config, fc fileConfig) {
	if fc.OpenAIAPIKey != "" {
		cfg.OpenAI.APIKey = fc.OpenAIAPIKey
	}
	if fc.OpenAIModel != "" {
		cfg.OpenAI.Model = fc.OpenAIModel
	}
	if fc.GenerateCount > 0 {
		cfg.Generate.Count = fc.GenerateCount
	}
	if fc.GenerateBatchSize > 0 {
		cfg.Generate.BatchSize = fc.GenerateBatchSize
	}
	if fc.GenerateMaxAttempts > 0 {
		cfg.Generate.MaxAttemptsPerBatch = fc.GenerateMaxAttempts
	}
	if fc.GenerateRetryCount >= 0 {
		cfg.Generate.RetryCount = fc.GenerateRetryCount
	}
	if fc.GenerateMaxLength > 0 {
		cfg.Generate.MaxLength = fc.GenerateMaxLength
	}
	if fc.GenerateMaxSyllables > 0 {
		cfg.Generate.MaxSyllables = fc.GenerateMaxSyllables
	}
	if fc.GenerateSuffix != "" {
		cfg.Generate.Suffix = fc.GenerateSuffix
	}
	if fc.GeneratePrefix != "" {
		cfg.Generate.Prefix = fc.GeneratePrefix
	}
	if fc.GenerateStyle != "" {
		cfg.Generate.Style = fc.GenerateStyle
	}
	if fc.PostgresDSN != "" {
		cfg.Postgres.DSN = fc.PostgresDSN
	}
}

func loadFile(path string) (fileConfig, error) {
	empty := fileConfig{
		GenerateMaxAttempts: -1,
		GenerateRetryCount:  -1,
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return empty, nil
		}
		return fileConfig{}, fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	cfg := empty
	section := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(line, ":") {
			section = strings.TrimSuffix(line, ":")
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return fileConfig{}, fmt.Errorf("parse config %q: invalid line %q", path, line)
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		switch section + "." + key {
		case "openai.api_key":
			cfg.OpenAIAPIKey = value
		case "openai.model":
			cfg.OpenAIModel = value
		case "generate.count":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fileConfig{}, fmt.Errorf("parse %s generate.count: %w", path, err)
			}
			cfg.GenerateCount = parsed
		case "generate.batch_size":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fileConfig{}, fmt.Errorf("parse %s generate.batch_size: %w", path, err)
			}
			cfg.GenerateBatchSize = parsed
		case "generate.max_attempts":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fileConfig{}, fmt.Errorf("parse %s generate.max_attempts: %w", path, err)
			}
			cfg.GenerateMaxAttempts = parsed
		case "generate.retry_count":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fileConfig{}, fmt.Errorf("parse %s generate.retry_count: %w", path, err)
			}
			cfg.GenerateRetryCount = parsed
		case "generate.max_length":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fileConfig{}, fmt.Errorf("parse %s generate.max_length: %w", path, err)
			}
			cfg.GenerateMaxLength = parsed
		case "generate.max_syllables":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fileConfig{}, fmt.Errorf("parse %s generate.max_syllables: %w", path, err)
			}
			cfg.GenerateMaxSyllables = parsed
		case "generate.suffix":
			cfg.GenerateSuffix = value
		case "generate.prefix":
			cfg.GeneratePrefix = value
		case "generate.style":
			cfg.GenerateStyle = value
		case "postgres.dsn":
			cfg.PostgresDSN = value
		default:
			return fileConfig{}, fmt.Errorf("parse config %q: unknown key %s.%s", path, section, key)
		}
	}
	if err := scanner.Err(); err != nil {
		return fileConfig{}, fmt.Errorf("read config %q: %w", path, err)
	}
	return cfg, nil
}
