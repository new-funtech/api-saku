package handlers

import (
	"net/http"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type LoanProductHandler struct {
	service services.LoanProductService
}

func NewLoanProductHandler(service services.LoanProductService) *LoanProductHandler {
	return &LoanProductHandler{service: service}
}

// GetAll godoc
// @Summary      List loan products
// @Tags         LoanProducts
// @Produce      json
// @Param        page      query int    false "Page" default(1)
// @Param        limit     query int    false "Limit" default(10)
// @Param        is_active query bool   false "Active filter"
// @Param        search    query string false "Search by name/code"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-products [get]
func (h *LoanProductHandler) GetAll(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c)
	filters := map[string]interface{}{}
	if v := c.Query("is_active"); v != "" {
		b := v == "true" || v == "1"
		filters["is_active"] = &b
	}
	if v := c.Query("search"); v != "" {
		filters["search"] = v
	}
	if v := c.Query("bujp_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filters["bujp_id"] = id
		}
	}
	// Tenant scoping: BUJP-scoped callers see only their tenant + global
	// (nil) products; Pusat sees everything.
	utils.ApplyBujpScope(c, filters)
	items, total, err := h.service.GetAll(c.Context(), page, limit, filters)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	totalPages, hasNext, hasPrev := utils.CalculatePaginationMeta(page, limit, total)
	return utils.SuccessResponseWithMeta(c, http.StatusOK, "loan products retrieved", items, &model.PaginationMeta{
		Page: page, Limit: limit, Total: total, TotalPages: totalPages, HasNext: hasNext, HasPrevious: hasPrev,
	})
}

// GetActive godoc
// @Summary      List active loan products available to current user
// @Tags         LoanProducts
// @Produce      json
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-products/active [get]
func (h *LoanProductHandler) GetActive(c *fiber.Ctx) error {
	var bujpID *uuid.UUID
	if v := c.Query("bujp_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			bujpID = &id
		}
	}
	items, err := h.service.GetActive(c.Context(), bujpID)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "active loan products retrieved", items)
}

// GetByID godoc
// @Summary      Get loan product by ID
// @Tags         LoanProducts
// @Produce      json
// @Param        id path string true "Loan Product UUID"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-products/{id} [get]
func (h *LoanProductHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	r, err := h.service.GetByID(c.Context(), id)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusNotFound, err.Error())
	}
	if !utils.CanAccessOptionalBujp(c, r.BujpID) {
		return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
	}
	return utils.SuccessResponse(c, http.StatusOK, "loan product retrieved", r)
}

// Create godoc
// @Summary      Create a loan product
// @Tags         LoanProducts
// @Accept       json
// @Produce      json
// @Param        body body model.CreateLoanProductRequest true "Body"
// @Success      201 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-products [post]
func (h *LoanProductHandler) Create(c *fiber.Ctx) error {
	var req model.CreateLoanProductRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid body")
	}
	// Tenant clamp: BUJP-scoped callers may only create products under
	// their own BUJP. Pusat may target any BUJP or global (nil).
	if role, _ := c.Locals("role").(string); !utils.IsPusat(role) {
		if scoped, ok := c.Locals("bujpID").(uuid.UUID); ok && scoped != uuid.Nil {
			req.BujpID = &scoped
		}
	}
	uid, _ := c.Locals("userID").(uuid.UUID)
	r, err := h.service.Create(c.Context(), &req, &uid)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusCreated, "loan product created", r)
}

// Update godoc
// @Summary      Update a loan product
// @Tags         LoanProducts
// @Accept       json
// @Produce      json
// @Param        id   path string true "ID"
// @Param        body body model.UpdateLoanProductRequest true "Body"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-products/{id} [put]
func (h *LoanProductHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	var req model.UpdateLoanProductRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid body")
	}
	existing, gerr := h.service.GetByID(c.Context(), id)
	if gerr != nil || existing == nil {
		return utils.ErrorResponse(c, http.StatusNotFound, "loan product not found")
	}
	if !utils.CanAccessOptionalBujp(c, existing.BujpID) {
		return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
	}
	if role, _ := c.Locals("role").(string); !utils.IsPusat(role) {
		req.BujpID = existing.BujpID
	}
	r, err := h.service.Update(c.Context(), id, &req)
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "loan product updated", r)
}

// Delete godoc
// @Summary      Delete a loan product
// @Tags         LoanProducts
// @Param        id path string true "ID"
// @Success      200 {object} model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/loan-products/{id} [delete]
func (h *LoanProductHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, "invalid id")
	}
	existing, gerr := h.service.GetByID(c.Context(), id)
	if gerr != nil || existing == nil {
		return utils.ErrorResponse(c, http.StatusNotFound, "loan product not found")
	}
	if !utils.CanAccessOptionalBujp(c, existing.BujpID) {
		return utils.ErrorResponse(c, http.StatusForbidden, "forbidden")
	}
	if err := h.service.Delete(c.Context(), id); err != nil {
		return utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	return utils.SuccessResponse(c, http.StatusOK, "loan product deleted", nil)
}
