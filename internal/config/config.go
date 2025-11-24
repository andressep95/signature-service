package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	AWSRegion                     string
	AWSAccessKeyID                string
	AWSSecretAccessKey            string
	S3BucketName                  string
	CompanyPrefix                 string
	PresignedURLExpirationMinutes int
	Port                          string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file if it exists (ignore error if file doesn't exist)
	_ = godotenv.Load()

	config := &Config{
		AWSRegion:          getEnv("AWS_REGION", "us-east-1"),
		AWSAccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
		AWSSecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		S3BucketName:       getEnv("S3_BUCKET_NAME", ""),
		CompanyPrefix:      getEnv("COMPANY_PREFIX", ""),
		Port:               getEnv("PORT", "8080"),
	}

	// Parse presigned URL expiration
	expirationStr := getEnv("PRESIGNED_URL_EXPIRATION_MINUTES", "3")
	expiration, err := strconv.Atoi(expirationStr)
	if err != nil {
		return nil, fmt.Errorf("invalid PRESIGNED_URL_EXPIRATION_MINUTES value: %w", err)
	}
	config.PresignedURLExpirationMinutes = expiration

	// Validate required fields
	if config.AWSAccessKeyID == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID is required")
	}
	if config.AWSSecretAccessKey == "" {
		return nil, fmt.Errorf("AWS_SECRET_ACCESS_KEY is required")
	}
	if config.S3BucketName == "" {
		return nil, fmt.Errorf("S3_BUCKET_NAME is required")
	}

	return config, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
