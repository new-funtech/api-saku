package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ProductHandler struct {
	service services.ProductService
}

func NewProductHandler(service services.ProductService) *ProductHandler {
	return &ProductHandler{service: service}
}

func (h *ProductHandler) moveTempToPermanent(product *model.ProductResponse) {
	if product.Image == "" || !strings.HasPrefix(product.Image, "temp/") {
		return
	}

	ctx := context.Background()
	filename := product.Image[strings.LastIndex(product.Image, "/")+1:]
	permanentKey := fmt.Sprintf("products/%s/%s", product.ID.String(), filename)

	if err := utils.MoveFileInS3(ctx, product.Image, permanentKey); err != nil {
		log.Printf("Warning: Failed to move image to permanent folder: %v", err)
		return
	}

	updateReq := model.UpdateProductRequest{Image: permanentKey}
	updated, err := h.service.UpdateProduct(ctx, product.ID, updateReq)
	if err != nil {
		log.Printf("Warning: Failed to update product image path: %v", err)
		return
	}
	*product = *updated
}

// GetAllProducts godoc
// @Summary      Get all products
// @Description  Retrieve a paginated list of all products sorted by newest first
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        page   query     int  false  "Page number" default(1)
// @Param        limit  query     int  false  "Items per page" default(10)
// @Success      200  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Router       /api/v1/products [get]
func (h *ProductHandler) GetAllProducts(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c)

	products, meta, err := h.service.GetAllProducts(c.Context(), page, limit)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: constants.ErrInternalServer,
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: constants.SuccessGetProducts,
		Data:    products,
		Meta:    meta,
	})
}

// GetProductByID godoc
// @Summary      Get product by ID
// @Description  Retrieve a single product by its UUID
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Product UUID"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Router       /api/v1/products/{id} [get]
func (h *ProductHandler) GetProductByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	product, err := h.service.GetProductByID(c.Context(), id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: constants.ErrProductNotFound,
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: constants.SuccessGetProduct,
		Data:    product,
	})
}

// CreateProduct godoc
// @Summary      Create a new product
// @Description  Create a new product with the provided data. If image is from temp folder, it will be moved to permanent folder.
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        request  body      model.CreateProductRequest  true  "Product data"
// @Success      201      {object}  model.APIResponse
// @Failure      400      {object}  model.APIResponse
// @Failure      500      {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/products [post]
func (h *ProductHandler) CreateProduct(c *fiber.Ctx) error {
	var req model.CreateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidRequest,
		})
	}

	product, err := h.service.CreateProduct(c.Context(), req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: constants.ErrInternalServer,
		})
	}

	// Move image from temp to permanent folder if needed
	h.moveTempToPermanent(product)

	return c.Status(http.StatusCreated).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusCreated,
		Message: constants.SuccessCreateProduct,
		Data:    product,
	})
}

// UpdateProduct godoc
// @Summary      Update a product
// @Description  Update an existing product by its UUID
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        id       path      string                      true  "Product UUID"
// @Param        request  body      model.UpdateProductRequest   true  "Product data"
// @Success      200      {object}  model.APIResponse
// @Failure      400      {object}  model.APIResponse
// @Failure      404      {object}  model.APIResponse
// @Failure      500      {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/products/{id} [put]
func (h *ProductHandler) UpdateProduct(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	var req model.UpdateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidRequest,
		})
	}

	product, err := h.service.UpdateProduct(c.Context(), id, req)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: constants.ErrProductNotFound,
		})
	}

	// Move image from temp to permanent folder if needed
	h.moveTempToPermanent(product)

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: constants.SuccessUpdateProduct,
		Data:    product,
	})
}

// DeleteProduct godoc
// @Summary      Delete a product
// @Description  Delete a product by its UUID
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Product UUID"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/products/{id} [delete]
func (h *ProductHandler) DeleteProduct(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	err = h.service.DeleteProduct(c.Context(), id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: constants.ErrProductNotFound,
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: constants.SuccessDeleteProduct,
	})
}

// UploadProductImage godoc
// @Summary      Upload product image
// @Description  Upload an image file to S3/MinIO storage (temporary folder)
// @Tags         Products
// @Accept       multipart/form-data
// @Produce      json
// @Param        image  formData  file  true  "Image file"
// @Success      200    {object}  model.APIResponse{data=object{image=string,preview_url=string,upload_expires_in=int,preview_expires_in=int}}
// @Failure      400    {object}  model.APIResponse
// @Failure      500    {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/products/upload [post]
func (h *ProductHandler) UploadProductImage(c *fiber.Ctx) error {
	file, err := c.FormFile("image")
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrImageRequired,
		})
	}

	// Validate file type
	contentType := file.Header.Get("Content-Type")
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}

	if !allowedTypes[contentType] {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidFileType,
		})
	}

	// Validate file size
	if file.Size > constants.MaxUploadSize {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrFileTooLarge,
		})
	}

	ctx := context.Background()
	objectKey, err := utils.UploadFileToS3(ctx, file, "temp")
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: constants.ErrUploadFailed + ": " + err.Error(),
		})
	}

	// Generate preview URL (valid for 7 days)
	previewURL, err := utils.GeneratePresignedURL(ctx, objectKey, 7*24*time.Hour)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: constants.ErrPreviewFailed + ": " + err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: constants.SuccessUploadImage,
		Data: fiber.Map{
			"image":              objectKey,
			"preview_url":        previewURL,
			"upload_expires_in":  86400,  // 24 hours
			"preview_expires_in": 604800, // 7 days
		},
	})
}
