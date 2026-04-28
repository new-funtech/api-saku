package handlers

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	_ "github.com/ganiramadhan/ganipedia/backend/internal/model" // for swagger annotations
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type UploadHandler struct{}

func NewUploadHandler() *UploadHandler {
	return &UploadHandler{}
}

// GetPresignedUploadURL generates a presigned URL for PUT request
// @Summary Get presigned URL for upload
// @Description Generate presigned URL for uploading files directly to S3/MinIO
// @Tags uploads
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body map[string]interface{} true "Upload request"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/uploads/presigned-url [post]
func (h *UploadHandler) GetPresignedUploadURL(c *fiber.Ctx) error {
	var req struct {
		FileName   string `json:"filename" validate:"required" example:"profile.jpg"`
		FolderType string `json:"folder_type" validate:"required,oneof=PERSONNELS LEAVES ATTENDANCE_CORRECTIONS PATROLS ATTENDANCES LOANS BUJPS" example:"PERSONNELS"`
		EntityID   string `json:"entity_id" validate:"required,uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
		Subfolder  string `json:"subfolder" example:"photo"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusBadRequest,
			"message": constants.ErrInvalidRequest,
			"data":    nil,
		})
	}

	// Default subfolder to "photo" if not provided
	if req.Subfolder == "" {
		req.Subfolder = "photo"
	}

	// Validate file extension
	ext := filepath.Ext(req.FileName)
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))

	allowedExtensions := []string{"jpg", "jpeg", "png", "gif", "webp"}
	if req.FolderType == "LOANS" || req.FolderType == "BUJPS" {
		allowedExtensions = append(allowedExtensions, "pdf")
	}

	valid := false
	for _, allowed := range allowedExtensions {
		if ext == allowed {
			valid = true
			break
		}
	}
	if !valid {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusBadRequest,
			"message": fmt.Sprintf("Only %s files are allowed", strings.Join(allowedExtensions, ", ")),
			"data":    nil,
		})
	}

	// Generate unique filename
	uniqueFilename := fmt.Sprintf("%s_%d.%s", uuid.New().String(), time.Now().Unix(), ext)

	// Build paths like Laravel: temp/PERSONNELS/entity_id/photo/filename
	tempPath := fmt.Sprintf("temp/%s/%s/%s/%s", req.FolderType, req.EntityID, req.Subfolder, uniqueFilename)
	finalPath := fmt.Sprintf("%s/%s/%s/%s", req.FolderType, req.EntityID, req.Subfolder, uniqueFilename)

	objectKey := tempPath

	// Determine content type from extension
	contentType := "application/octet-stream"
	switch ext {
	case "jpg", "jpeg":
		contentType = "image/jpeg"
	case "png":
		contentType = "image/png"
	case "gif":
		contentType = "image/gif"
	case "webp":
		contentType = "image/webp"
	case "pdf":
		contentType = "application/pdf"
	}

	// Generate presigned URL for PUT
	ctx := context.Background()
	presignClient := s3.NewPresignClient(config.S3Client)

	presignedURL, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(config.S3Bucket),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 60 * time.Minute // URL valid for 60 minutes
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusInternalServerError,
			"message": constants.ErrGeneratePresignedURL,
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"code":    fiber.StatusOK,
		"message": constants.SuccessGeneratePresignedURL,
		"data": fiber.Map{
			"upload_url": presignedURL.URL,
			"file_path":  uniqueFilename,
			"temp_path":  tempPath,
			"final_path": finalPath,
			"expires_in": 3600, // 60 minutes in seconds
		},
	})
}

// GetPresignedDownloadURL generates a presigned URL for GET request
// @Summary Get presigned URL for download
// @Description Generate presigned URL for downloading files from S3/MinIO
// @Tags uploads
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body map[string]interface{} true "Download request"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/uploads/presigned-download-url [post]
func (h *UploadHandler) GetPresignedDownloadURL(c *fiber.Ctx) error {
	var req struct {
		ObjectKey string `json:"object_key" validate:"required" example:"temp/profile-abc123.jpg"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusBadRequest,
			"message": constants.ErrInvalidRequest,
			"data":    nil,
		})
	}

	ctx := context.Background()
	downloadURL, err := utils.GeneratePresignedURL(ctx, req.ObjectKey, 1*time.Hour)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusInternalServerError,
			"message": constants.ErrGeneratePresignedURL,
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"code":    fiber.StatusOK,
		"message": constants.SuccessGeneratePresignedURL,
		"data": fiber.Map{
			"download_url": downloadURL,
			"object_key":   req.ObjectKey,
			"expires_in":   3600, // 1 hour in seconds
		},
	})
}

// DirectUpload handles direct file upload
// @Summary Upload file directly
// @Description Upload file directly to S3/MinIO
// @Tags uploads
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "File to upload"
// @Param folder formData string true "Upload folder" Enums(temp, users, personnels, products, documents)
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/uploads/direct [post]
func (h *UploadHandler) DirectUpload(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusBadRequest,
			"message": constants.ErrImageRequired,
			"data":    nil,
		})
	}

	folder := c.FormValue("folder", "temp")

	// Validate folder
	validFolders := map[string]bool{
		"temp": true, "users": true, "personnels": true,
		"products": true, "documents": true,
	}
	if !validFolders[folder] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusBadRequest,
			"message": "Invalid folder. Must be one of: temp, users, personnels, products, documents",
			"data":    nil,
		})
	}

	// Validate file size
	if file.Size > constants.MaxUploadSize {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusBadRequest,
			"message": constants.ErrFileTooLarge,
			"data":    nil,
		})
	}

	// Validate content type
	contentType := file.Header.Get("Content-Type")
	if !isValidContentType(contentType) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusBadRequest,
			"message": constants.ErrInvalidFileType,
			"data":    nil,
		})
	}

	// Upload to S3
	ctx := context.Background()
	objectKey, err := utils.UploadFileToS3(ctx, file, folder)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusInternalServerError,
			"message": constants.ErrUploadFailed,
			"data":    nil,
		})
	}

	// Generate preview URL
	previewURL, err := utils.GeneratePresignedURL(ctx, objectKey, 1*time.Hour)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusInternalServerError,
			"message": constants.ErrPreviewFailed,
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"code":    fiber.StatusOK,
		"message": constants.SuccessUploadImage,
		"data": fiber.Map{
			"object_key":   objectKey,
			"preview_url":  previewURL,
			"file_name":    file.Filename,
			"file_size":    file.Size,
			"content_type": contentType,
		},
	})
}

// DeleteFile deletes a file from S3
// @Summary Delete file
// @Description Delete file from S3/MinIO
// @Tags uploads
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body map[string]interface{} true "Delete request"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/uploads [delete]
func (h *UploadHandler) DeleteFile(c *fiber.Ctx) error {
	var req struct {
		ObjectKey string `json:"object_key" validate:"required" example:"temp/profile-abc123.jpg"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusBadRequest,
			"message": constants.ErrInvalidRequest,
			"data":    nil,
		})
	}

	ctx := context.Background()
	if err := utils.DeleteFileFromS3(ctx, req.ObjectKey); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusInternalServerError,
			"message": constants.ErrDeleteFailed,
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"code":    fiber.StatusOK,
		"message": constants.SuccessDeleteFile,
		"data":    nil,
	})
}

// MoveFile moves a file from one location to another in S3
// @Summary Move file
// @Description Move file from one location to another in S3/MinIO
// @Tags uploads
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body map[string]interface{} true "Move request"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /api/v1/uploads/move [post]
func (h *UploadHandler) MoveFile(c *fiber.Ctx) error {
	var req struct {
		SourceKey string `json:"source_key" validate:"required" example:"temp/profile-abc123.jpg"`
		DestKey   string `json:"dest_key" validate:"required" example:"users/profile-abc123.jpg"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusBadRequest,
			"message": constants.ErrInvalidRequest,
			"data":    nil,
		})
	}

	ctx := context.Background()
	if err := utils.MoveFileInS3(ctx, req.SourceKey, req.DestKey); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"code":    fiber.StatusInternalServerError,
			"message": constants.ErrMoveFailed,
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"code":    fiber.StatusOK,
		"message": constants.SuccessMoveFile,
		"data": fiber.Map{
			"new_object_key": req.DestKey,
		},
	})
}

func isValidContentType(contentType string) bool {
	// Normalize content type: trim whitespace and convert to lowercase
	contentType = strings.ToLower(strings.TrimSpace(contentType))

	// Remove any charset or other parameters
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	validTypes := map[string]bool{
		// Images
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,

		// PDF
		"application/pdf": true,

		// Word documents
		"application/msword": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"application/doc":  true, // Some browsers may send this
		"application/docx": true, // Some browsers may send this

		// Excel spreadsheets
		"application/vnd.ms-excel": true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
		"application/xls":  true, // Some browsers may send this
		"application/xlsx": true, // Some browsers may send this
	}
	return validTypes[contentType]
}

func isValidFileExtension(file *multipart.FileHeader) bool {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	validExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	}
	return validExts[ext]
}

func getMaxFileSize(contentType string) int64 {
	// Images have 5MB limit, documents have 10MB limit
	if strings.HasPrefix(contentType, "image/") {
		return constants.MaxImageSize
	}
	return constants.MaxDocumentSize
}
