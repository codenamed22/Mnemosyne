package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	Port          int    `json:"port"`
	StoragePath   string `json:"storage_path"`
	BindAddress   string `json:"bind_address"`
	MaxUploadMB   int64  `json:"max_upload_mb"`
	SessionExpHrs int    `json:"session_expiry_hours"`
	EnableHTTPS   bool   `json:"enable_https"`
	CertPath      string `json:"cert_path"`
	KeyPath       string `json:"key_path"`
	UseMkcert     bool   `json:"use_mkcert"` // Set to true if using mkcert certificates (suppresses warning messages)

	// Photo Selector / AI Features
	EmbeddingServiceURL string `json:"embedding_service_url"` // CLIP embedding service URL
	SimilarityThreshold float64 `json:"similarity_threshold"` // Threshold for grouping similar photos (0-1)

	// LLM Configuration
	LLMProvider        string `json:"llm_provider"`         // openai, azure, gemini, custom
	LLMAPIKey          string `json:"llm_api_key"`          // API key for the LLM provider
	LLMBaseURL         string `json:"llm_base_url"`         // Base URL (for Azure/custom providers)
	LLMModel           string `json:"llm_model"`            // Model name (e.g., gpt-4o, gemini-1.5-pro)
	LLMAzureDeployment string `json:"llm_azure_deployment"` // Azure deployment name
	LLMAzureAPIVersion string `json:"llm_azure_api_version"` // Azure API version
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Port:          8080,
		StoragePath:   "./data",
		BindAddress:   "0.0.0.0",
		MaxUploadMB:   50,
		SessionExpHrs: 24,
		EnableHTTPS:   true,
		CertPath:      "./certs/server.crt",
		KeyPath:       "./certs/server.key",

		// Photo Selector defaults
		EmbeddingServiceURL: "http://127.0.0.1:8081",
		SimilarityThreshold: 0.75, // 75% similarity

		// LLM defaults (unconfigured)
		LLMProvider:        "",
		LLMAPIKey:          "",
		LLMBaseURL:         "",
		LLMModel:           "",
		LLMAzureDeployment: "",
		LLMAzureAPIVersion: "2024-02-15-preview",
	}
}

// GetLLMConfig returns the LLM configuration
func (c *Config) GetLLMConfig() LLMConfig {
	return LLMConfig{
		Provider:        LLMProvider(c.LLMProvider),
		APIKey:          c.LLMAPIKey,
		BaseURL:         c.LLMBaseURL,
		Model:           c.LLMModel,
		AzureDeployment: c.LLMAzureDeployment,
		AzureAPIVersion: c.LLMAzureAPIVersion,
	}
}

// IsLLMConfigured checks if LLM is configured
func (c *Config) IsLLMConfigured() bool {
	return c.LLMProvider != "" && c.LLMAPIKey != ""
}

// LoadConfig loads configuration from file or creates default
func LoadConfig(path string) (*Config, error) {
	// If config doesn't exist, create default
	if _, err := os.Stat(path); os.IsNotExist(err) {
		config := DefaultConfig()

		fmt.Println("No config found. Creating default configuration...")

		// Save config
		if err := config.Save(path); err != nil {
			return nil, fmt.Errorf("failed to save config: %v", err)
		}

		fmt.Printf("Configuration saved to %s\n", path)
		return config, nil
	}

	// Load existing config
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %v", err)
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	return config, nil
}

// Save writes the configuration to a file
func (c *Config) Save(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}

	if c.StoragePath == "" {
		return fmt.Errorf("storage_path cannot be empty")
	}

	if c.MaxUploadMB < 1 {
		return fmt.Errorf("max_upload_mb must be at least 1")
	}

	return nil
}

// EnsureDirectories creates necessary directories
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.StoragePath,
		filepath.Join(c.StoragePath, "users"),
	}

	if c.EnableHTTPS {
		certDir := filepath.Dir(c.CertPath)
		dirs = append(dirs, certDir)
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}

	return nil
}
