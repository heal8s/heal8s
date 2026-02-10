package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	GitHub     GitHubConfig     `yaml:"github"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	Processor  ProcessorConfig  `yaml:"processor"`
}

// GitHubConfig holds GitHub App configuration
type GitHubConfig struct {
	AppID          int64  `yaml:"appID"`
	InstallationID int64  `yaml:"installationID"`
	PrivateKeyPath string `yaml:"privateKeyPath"`
}

// KubernetesConfig holds Kubernetes client configuration
type KubernetesConfig struct {
	Kubeconfig string `yaml:"kubeconfig"`
	Namespace  string `yaml:"namespace"`
}

// ProcessorConfig holds processor configuration
type ProcessorConfig struct {
	PollInterval string `yaml:"pollInterval"`
	BatchSize    int    `yaml:"batchSize"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.Processor.PollInterval == "" {
		config.Processor.PollInterval = "10s"
	}
	if config.Processor.BatchSize == 0 {
		config.Processor.BatchSize = 10
	}
	if config.Kubernetes.Namespace == "" {
		config.Kubernetes.Namespace = "heal8s-system"
	}

	return &config, nil
}

// LoadFromEnv loads configuration from environment variables (for production)
func LoadFromEnv() (*Config, error) {
	config := &Config{
		GitHub: GitHubConfig{
			AppID:          getEnvInt64("GITHUB_APP_ID", 0),
			InstallationID: getEnvInt64("GITHUB_INSTALLATION_ID", 0),
			PrivateKeyPath: getEnv("GITHUB_PRIVATE_KEY_PATH", "/secrets/github-app.pem"),
		},
		Kubernetes: KubernetesConfig{
			Kubeconfig: getEnv("KUBECONFIG", ""),
			Namespace:  getEnv("K8S_NAMESPACE", "heal8s-system"),
		},
		Processor: ProcessorConfig{
			PollInterval: getEnv("POLL_INTERVAL", "10s"),
			BatchSize:    getEnvInt("BATCH_SIZE", 10),
		},
	}

	// Validate required fields
	if config.GitHub.AppID == 0 {
		return nil, fmt.Errorf("GITHUB_APP_ID is required")
	}
	if config.GitHub.InstallationID == 0 {
		return nil, fmt.Errorf("GITHUB_INSTALLATION_ID is required")
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		var result int64
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}
