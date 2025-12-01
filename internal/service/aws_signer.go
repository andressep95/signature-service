package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// AWSSigner handles AWS Signature Version 4 signing
type AWSSigner struct {
	accessKey string
	secretKey string
	region    string
	service   string
}

// NewAWSSigner creates a new AWS signer
func NewAWSSigner(accessKey, secretKey, region, service string) *AWSSigner {
	return &AWSSigner{
		accessKey: accessKey,
		secretKey: secretKey,
		region:    region,
		service:   service,
	}
}

// GeneratePresignedPutURL generates a presigned URL for PUT operations
func (s *AWSSigner) GeneratePresignedPutURL(bucket, key, contentType string, metadata map[string]string, expiration time.Duration) (string, error) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	// Build host
	host := fmt.Sprintf("%s.s3.%s.amazonaws.com", bucket, s.region)

	// Canonical URI
	canonicalURI := "/" + key

	// Build canonical headers - start with host
	headers := map[string]string{
		"host": host,
	}

	// Add metadata headers (x-amz-meta-*)
	for k, v := range metadata {
		// Normalize header key to lowercase and replace underscores with hyphens (HTTP standard)
		normalizedKey := strings.ReplaceAll(k, "_", "-")
		headerKey := strings.ToLower(fmt.Sprintf("x-amz-meta-%s", normalizedKey))
		// Normalize header value - trim whitespace and collapse multiple spaces
		headerValue := strings.TrimSpace(v)
		// Replace multiple consecutive spaces with single space
		headerValue = strings.Join(strings.Fields(headerValue), " ")
		headers[headerKey] = headerValue
	}

	// Build sorted canonical headers and signed headers list
	headerKeys := make([]string, 0, len(headers))
	for k := range headers {
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)

	var canonicalHeadersParts []string
	for _, k := range headerKeys {
		// Trim whitespace from header values and ensure proper formatting
		headerValue := strings.TrimSpace(headers[k])
		canonicalHeadersParts = append(canonicalHeadersParts, fmt.Sprintf("%s:%s", k, headerValue))
	}
	canonicalHeaders := strings.Join(canonicalHeadersParts, "\n") + "\n"
	signedHeaders := strings.Join(headerKeys, ";")

	// Build query parameters
	// Note: Content-Type should NOT be in query params for presigned URLs
	// It must be included as a header when making the actual PUT request
	queryParams := map[string]string{
		"X-Amz-Algorithm":     "AWS4-HMAC-SHA256",
		"X-Amz-Credential":    fmt.Sprintf("%s/%s/%s/%s/aws4_request", s.accessKey, dateStamp, s.region, s.service),
		"X-Amz-Date":          amzDate,
		"X-Amz-Expires":       fmt.Sprintf("%d", int(expiration.Seconds())),
		"X-Amz-SignedHeaders": signedHeaders,
	}

	// Build canonical query string
	canonicalQueryString := s.buildCanonicalQueryString(queryParams)

	// Payload hash for presigned URLs is always UNSIGNED-PAYLOAD
	payloadHash := "UNSIGNED-PAYLOAD"

	// Build canonical request
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		"PUT",
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	// Build string to sign
	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, s.region, s.service)
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		amzDate,
		credentialScope,
		s.hash(canonicalRequest),
	)

	// Calculate signature
	signingKey := s.getSignatureKey(s.secretKey, dateStamp, s.region, s.service)
	signature := s.hmacSHA256Hex(signingKey, stringToSign)

	// Add signature to query parameters
	queryParams["X-Amz-Signature"] = signature

	// Build final URL - DON'T encode slashes to avoid double-encoding by HTTP clients
	finalQueryString := s.buildFinalQueryString(queryParams)
	presignedURL := fmt.Sprintf("https://%s%s?%s", host, canonicalURI, finalQueryString)

	return presignedURL, nil
}

// buildCanonicalQueryString builds a canonical query string from parameters
func (s *AWSSigner) buildCanonicalQueryString(params map[string]string) string {
	// Sort keys
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build query string
	var parts []string
	for _, k := range keys {
		// Encode all values including slashes - AWS expects them encoded in the canonical request
		encodedValue := s.uriEncode(params[k], true)
		parts = append(parts, fmt.Sprintf("%s=%s", s.uriEncode(k, true), encodedValue))
	}

	return strings.Join(parts, "&")
}

// buildFinalQueryString builds the final query string for the URL (without encoding slashes in credential)
func (s *AWSSigner) buildFinalQueryString(params map[string]string) string {
	// Sort keys
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build query string - don't encode slashes in X-Amz-Credential to avoid double-encoding
	var parts []string
	for _, k := range keys {
		encodedKey := s.uriEncode(k, true)
		var encodedValue string
		if k == "X-Amz-Credential" {
			// Don't encode slashes in credential for the final URL
			encodedValue = s.uriEncode(params[k], false)
		} else {
			encodedValue = s.uriEncode(params[k], true)
		}
		parts = append(parts, fmt.Sprintf("%s=%s", encodedKey, encodedValue))
	}

	return strings.Join(parts, "&")
}

// uriEncode encodes a string for use in a URL
func (s *AWSSigner) uriEncode(input string, encodeSlash bool) string {
	var result strings.Builder
	for _, r := range input {
		c := string(r)
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			c == "_" || c == "-" || c == "~" || c == "." {
			result.WriteRune(r)
		} else if c == "/" && !encodeSlash {
			result.WriteString("/")
		} else {
			result.WriteString(fmt.Sprintf("%%%02X", r))
		}
	}
	return result.String()
}

// hash returns the SHA256 hash of the input
func (s *AWSSigner) hash(text string) string {
	h := sha256.New()
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}

// hmacSHA256 computes HMAC-SHA256
func (s *AWSSigner) hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// hmacSHA256Hex computes HMAC-SHA256 and returns hex string
func (s *AWSSigner) hmacSHA256Hex(key []byte, data string) string {
	return hex.EncodeToString(s.hmacSHA256(key, data))
}

// getSignatureKey derives the signing key
func (s *AWSSigner) getSignatureKey(secretKey, dateStamp, region, service string) []byte {
	kSecret := []byte("AWS4" + secretKey)
	kDate := s.hmacSHA256(kSecret, dateStamp)
	kRegion := s.hmacSHA256(kDate, region)
	kService := s.hmacSHA256(kRegion, service)
	kSigning := s.hmacSHA256(kService, "aws4_request")
	return kSigning
}
