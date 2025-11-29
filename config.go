package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"
)

// Config holds the application configuration
type Config struct {
	Password       string `json:"password"`
	PasswordHash   string `json:"password_hash,omitempty"`
	Port           int    `json:"port"`
	StoragePath    string `json:"storage_path"`
	BindAddress    string `json:"bind_address"`
	MaxUploadMB    int64  `json:"max_upload_mb"`
	SessionExpHrs  int    `json:"session_expiry_hours"`
	EnableHTTPS    bool   `json:"enable_https"`
	CertPath       string `json:"cert_path"`
	KeyPath        string `json:"key_path"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Port:          8080,
		StoragePath:   "./photos",
		BindAddress:   "0.0.0.0",
		MaxUploadMB:   50,
		SessionExpHrs: 24,
		EnableHTTPS:   true,
		CertPath:      "./certs/server.crt",
		KeyPath:       "./certs/server.key",
	}
}

// LoadConfig loads configuration from file or creates default
func LoadConfig(path string) (*Config, error) {
	// If config doesn't exist, create default
	if _, err := os.Stat(path); os.IsNotExist(err) {
		config := DefaultConfig()
		
		// Prompt for password
		fmt.Println("No config found. Creating new configuration...")
		fmt.Print("Enter password for photo cloud (or press Enter for random password): ")
		var password string
		fmt.Scanln(&password)
		
		if password == "" {
			password = generateRandomPassword(16)
			fmt.Printf("Generated random password: %s\n", password)
			fmt.Println("IMPORTANT: Save this password! It won't be shown again.")
		}
		
		// Hash password immediately - don't store plaintext
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %v", err)
		}
		config.PasswordHash = string(hash)
		// Don't store plaintext password in config file
		config.Password = ""
		
		// Save config
		if err := config.Save(path); err != nil {
			return nil, fmt.Errorf("failed to save config: %v", err)
		}
		
		// Set password back for runtime use (session manager needs it)
		config.Password = password
		
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
	
	return os.WriteFile(path, data, 0600) // Only owner can read/write
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	
	if c.StoragePath == "" {
		return fmt.Errorf("storage_path cannot be empty")
	}
	
	if c.Password == "" && c.PasswordHash == "" {
		return fmt.Errorf("password or password_hash must be set")
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
		filepath.Join(c.StoragePath, "originals"),
		filepath.Join(c.StoragePath, "thumbnails"),
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

