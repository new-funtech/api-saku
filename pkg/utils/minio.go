package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	"github.com/google/uuid"
)

func UploadFileToS3(ctx context.Context, file *multipart.FileHeader, folder string) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%s-%s%s", strings.TrimSuffix(filepath.Base(file.Filename), ext), uuid.New().String()[:8], ext)

	objectKey := fmt.Sprintf("%s/%s", folder, filename)

	fileBytes, err := io.ReadAll(src)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	_, err = config.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(config.S3Bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(file.Header.Get("Content-Type")),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file to S3: %w", err)
	}

	return objectKey, nil
}

func GeneratePresignedURL(ctx context.Context, objectKey string, expiration time.Duration) (string, error) {
	if objectKey == "" {
		return "", nil
	}

	presignClient := s3.NewPresignClient(config.S3Client)

	presignedURL, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(config.S3Bucket),
		Key:    aws.String(objectKey),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiration
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedURL.URL, nil
}

func MoveFileInS3(ctx context.Context, sourceKey, destKey string) error {
	if sourceKey == "" || destKey == "" {
		return fmt.Errorf("source and destination keys are required")
	}

	copySource := fmt.Sprintf("%s/%s", config.S3Bucket, sourceKey)
	_, err := config.S3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(config.S3Bucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(destKey),
	})
	if err != nil {
		return fmt.Errorf("failed to copy file in S3: %w", err)
	}

	err = DeleteFileFromS3(ctx, sourceKey)
	if err != nil {
		return fmt.Errorf("failed to delete source file: %w", err)
	}

	return nil
}

func DeleteFileFromS3(ctx context.Context, objectKey string) error {
	if objectKey == "" {
		return nil
	}

	_, err := config.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(config.S3Bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete file from S3: %w", err)
	}

	return nil
}

func ListOldTempFiles(ctx context.Context, olderThan time.Duration) ([]string, error) {
	var oldFiles []string
	cutoffTime := time.Now().Add(-olderThan)

	result, err := config.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(config.S3Bucket),
		Prefix: aws.String("temp/"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	for _, obj := range result.Contents {
		if obj.LastModified.Before(cutoffTime) {
			oldFiles = append(oldFiles, *obj.Key)
		}
	}

	return oldFiles, nil
}
