package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds the application configuration
type Config struct {
	ServerPort int

	// Microsoft To Do configuration
	MicrosoftClientID     string
	MicrosoftClientSecret string
	MicrosoftAccessToken  string

	// Google Tasks configuration
	GoogleClientID     string
	GoogleClientSecret string
	GoogleAccessToken  string

	// Todoist configuration
	TodoistAPIKey string

	// Notion configuration
	NotionIntegrationToken string
	NotionDatabaseID       string

	// Logging
	LogLevel string
}

// LoadConfig loads the configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		// If .env file doesn't exist, continue with environment variables only
		fmt.Println("No .env file found, using environment variables only")
	}

	config := &Config{
		ServerPort: getEnvAsInt("SERVER_PORT", 8080),

		MicrosoftClientID:     getEnv("MICROSOFT_CLIENT_ID", ""),
		MicrosoftClientSecret: getEnv("MICROSOFT_CLIENT_SECRET", ""),
		MicrosoftAccessToken:  getEnv("MICROSOFT_ACCESS_TOKEN", ""),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleAccessToken:  getEnv("GOOGLE_ACCESS_TOKEN", ""),

		TodoistAPIKey: getEnv("TODOIST_API_KEY", ""),

		NotionIntegrationToken: getEnv("NOTION_INTEGRATION_TOKEN", ""),
		NotionDatabaseID:       getEnv("NOTION_DATABASE_ID", ""),

		LogLevel: getEnv("LOG_LEVEL", "info"),
	}

	return config, nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		_, err := fmt.Sscanf(value, "%d", &result)
		if err != nil {
			return defaultValue
		}
		return result
	}
	return defaultValue
}
