package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	"github.com/google/uuid"
)

// UploadBase64DataURL accepts a string that may either be a data URL
// ("data:image/jpeg;base64,...."), a raw base64 payload, or an existing
// object key. When a base64 payload is detected the content is uploaded to
// S3/MinIO under the given folder and the resulting object key is returned.
// When the input does not look like base64 data the original value is
// returned unchanged so existing keys remain untouched.
func UploadBase64DataURL(ctx context.Context, value, folder string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", nil
	}

	// data URL form
	contentType := "application/octet-stream"
	ext := ".bin"
	payload := v

	if strings.HasPrefix(v, "data:") {
		commaIdx := strings.Index(v, ",")
		if commaIdx < 0 {
			return value, nil
		}
		header := v[5:commaIdx] // strip "data:"
		payload = v[commaIdx+1:]

		// header e.g. "image/jpeg;base64"
		parts := strings.Split(header, ";")
		if len(parts) > 0 && parts[0] != "" {
			contentType = parts[0]
			if i := strings.Index(contentType, "/"); i > -1 {
				ext = "." + contentType[i+1:]
			}
		}
	} else {
		// Heuristic: only treat very long base64-looking strings as uploads.
		// Otherwise assume the value is already an object key.
		if len(v) < 256 || strings.ContainsAny(v, "/ ") {
			return value, nil
		}
	}

	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		// Fallback: try URL-safe decoding before giving up.
		data, err = base64.URLEncoding.DecodeString(payload)
		if err != nil {
			return value, nil
		}
	}

	// Hard cap on decoded payload to prevent OOM / runaway uploads.
	// 8MB covers high-quality phone selfies but rejects abusive payloads.
	const maxBase64Bytes = 8 * 1024 * 1024
	if len(data) > maxBase64Bytes {
		return "", fmt.Errorf("ukuran gambar terlalu besar (maks 8 MB)")
	}

	key := fmt.Sprintf("%s/%s%s", folder, uuid.New().String(), ext)

	if config.S3Client == nil {
		return "", fmt.Errorf("s3 client not configured")
	}

	if _, err := config.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(config.S3Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}); err != nil {
		return "", fmt.Errorf("failed to upload base64 payload to S3: %w", err)
	}

	return key, nil
}
