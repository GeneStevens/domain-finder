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
	baseConfigName  = "domainfinder.yaml"
	localConfigName = "domainfinder.local.yaml"
)

// Config is the resolved runtime configuration.
type Config struct {
	OpenAI   OpenAIConfig
	Generate GenerateConfig
}

type OpenAIConfig struct {
	APIKey string
	Model  string
}

type GenerateConfig struct {
	Count     int
	BatchSize int
}

// CLIOverrides are the CLI-provided config overrides.
type CLIOverrides struct {
	OpenAIModel       string
	GenerateCount     int
	GenerateBatchSize int
}

type fileConfig struct {
	OpenAIAPIKey      string
	OpenAIModel       string
	GenerateCount     int
	GenerateBatchSize int
}

// Load resolves configuration using precedence:
// CLI > environment > local YAML > base YAML > built-in defaults.
func Load(dir string, lookupEnv func(string) (string, bool), cli CLIOverrides) (Config, error) {
	cfg := Config{
		OpenAI: OpenAIConfig{
			Model: "gpt-4o-mini",
		},
		Generate: GenerateConfig{
			Count:     20,
			BatchSize: 10,
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

	if cli.OpenAIModel != "" {
		cfg.OpenAI.Model = cli.OpenAIModel
	}
	if cli.GenerateCount > 0 {
		cfg.Generate.Count = cli.GenerateCount
	}
	if cli.GenerateBatchSize > 0 {
		cfg.Generate.BatchSize = cli.GenerateBatchSize
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
}

func loadFile(path string) (fileConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fileConfig{}, nil
		}
		return fileConfig{}, fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var cfg fileConfig
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
		default:
			return fileConfig{}, fmt.Errorf("parse config %q: unknown key %s.%s", path, section, key)
		}
	}
	if err := scanner.Err(); err != nil {
		return fileConfig{}, fmt.Errorf("read config %q: %w", path, err)
	}
	return cfg, nil
}
