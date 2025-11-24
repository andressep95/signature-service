package service

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/andressep95/aws-backup-bridge/signer-service/internal/config"
)

// S3Service handles S3 operations
type S3Service struct {
	client        *s3.Client
	signer        *AWSSigner
	bucketName    string
	companyPrefix string
	region        string
	expiration    time.Duration
}

// NewS3Service creates a new S3 service instance
func NewS3Service(cfg *config.Config) (*S3Service, error) {
	// Create AWS config with explicit credentials using LoadDefaultConfig
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(cfg.AWSRegion),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AWSAccessKeyID,
			cfg.AWSSecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg)

	// Create manual signer for presigned URLs
	signer := NewAWSSigner(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, cfg.AWSRegion, "s3")

	return &S3Service{
		client:        client,
		signer:        signer,
		bucketName:    cfg.S3BucketName,
		companyPrefix: cfg.CompanyPrefix,
		region:        cfg.AWSRegion,
		expiration:    time.Duration(cfg.PresignedURLExpirationMinutes) * time.Minute,
	}, nil
}

// buildObjectKey constructs the full object key with company prefix
// If company prefix is empty, returns just the objectKey without leading slash
func (s *S3Service) buildObjectKey(objectKey string) string {
	if s.companyPrefix == "" {
		return objectKey
	}
	return fmt.Sprintf("%s/%s", s.companyPrefix, objectKey)
}

// buildTimestampedPath constructs object path with inputs/date/time/ prefix
// Format: inputs/YYYY-MM-DD/HH-MM-SS/filename
func (s *S3Service) buildTimestampedPath(filename string) string {
	now := time.Now().UTC()

	// Format: inputs/2024-01-16/14-30-00/filename
	datePart := now.Format("2006-01-02")     // YYYY-MM-DD
	timePart := now.Format("15-04-05")       // HH-MM-SS

	path := fmt.Sprintf("inputs/%s/%s/%s", datePart, timePart, filename)
	return path
}

// SearchObjectByFilename searches for a file by name in the company's prefix
func (s *S3Service) SearchObjectByFilename(ctx context.Context, filename string) (bool, string, error) {
	// Build search prefix
	var searchPrefix string
	if s.companyPrefix == "" {
		searchPrefix = "inputs/" // Search in inputs folder when no company prefix
	} else {
		searchPrefix = s.companyPrefix + "/"
	}

	// List all objects in the search prefix
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(searchPrefix),
	}

	result, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return false, "", fmt.Errorf("failed to list objects: %w", err)
	}

	// Search for the filename in all object keys
	for _, obj := range result.Contents {
		if obj.Key != nil {
			// Check if the key ends with the filename
			key := *obj.Key
			if len(key) >= len(filename) && key[len(key)-len(filename):] == filename {
				return true, key, nil
			}
		}
	}

	return false, "", nil
}

// GeneratePresignedPutURL generates a presigned URL for uploading an object
// Returns: (presignedURL, fullObjectPath, error)
func (s *S3Service) GeneratePresignedPutURL(ctx context.Context, filename string, contentType string, metadata map[string]string) (string, string, error) {
	// Build timestamped path: inputs/date/time/filename
	timestampedPath := s.buildTimestampedPath(filename)

	// Build full object key with company prefix
	fullKey := s.buildObjectKey(timestampedPath)

	// Use manual signer to generate presigned URL
	presignedURL, err := s.signer.GeneratePresignedPutURL(s.bucketName, fullKey, contentType, metadata, s.expiration)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedURL, fullKey, nil
}
