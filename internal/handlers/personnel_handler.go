package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type PersonnelHandler struct {
	service       services.PersonnelService
	importService services.PersonnelImportService
	userRepo      repository.UserRepository
}

func NewPersonnelHandler(service services.PersonnelService, importService services.PersonnelImportService, userRepo repository.UserRepository) *PersonnelHandler {
	return &PersonnelHandler{service: service, importService: importService, userRepo: userRepo}
}

// GetAllPersonnels godoc
// @Summary      Get all personnels
// @Description  Retrieve a paginated list of all personnels
// @Tags         Personnels
// @Accept       json
// @Produce      json
// @Param        page    query     int     false  "Page number" default(1)
// @Param        limit   query     int     false  "Items per page" default(10)
// @Param        bujp_id query     string  false  "Filter by BUJP ID"
// @Param        status  query     string  false  "Filter by status"
// @Param        search  query     string  false  "Search by ID number, name, or phone"
// @Success      200  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/personnels [get]
func (h *PersonnelHandler) GetAllPersonnels(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c)

	// Enforce maximum limit to prevent database overload
	if limit > 100 {
		limit = 100
	}
	if page < 1 {
		page = 1
	}

	filters := make(map[string]interface{})
	if bujpIDStr := c.Query("bujp_id"); bujpIDStr != "" {
		if bujpID, err := uuid.Parse(bujpIDStr); err == nil {
			filters["bujp_id"] = bujpID
		}
	}
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if search := c.Query("search"); search != "" {
		filters["search"] = search
	}
	if unassigned := c.Query("unassigned_only"); unassigned == "true" || unassigned == "1" {
		filters["unassigned_only"] = true
	}

	utils.ApplyBujpScope(c, filters)

	personnels, total, err := h.service.GetAll(c.Context(), page, limit, filters)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: constants.ErrInternalServer,
		})
	}

	totalPages := (total + int64(limit) - 1) / int64(limit)

	meta := &model.PaginationMeta{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  int(totalPages),
		HasNext:     page < int(totalPages),
		HasPrevious: page > 1,
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Successfully retrieved personnels",
		Data:    personnels,
		Meta:    meta,
	})
}

// GetPersonnelByID godoc
// @Summary      Get personnel by ID
// @Description  Retrieve a single personnel by its UUID
// @Tags         Personnels
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Personnel UUID"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/personnels/{id} [get]
func (h *PersonnelHandler) GetPersonnelByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	personnel, err := h.service.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: "Personnel not found",
		})
	}

	if !utils.CanAccessPersonnel(c, personnel.ID, personnel.BujpID) {
		return forbiddenResponse(c)
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Successfully retrieved personnel",
		Data:    personnel,
	})
}

// CreatePersonnel godoc
// @Summary      Create a new personnel
// @Description  Create a new personnel with the provided data
// @Tags         Personnels
// @Accept       json
// @Produce      json
// @Param        request  body      model.CreatePersonnelRequest  true  "Personnel data"
// @Success      201  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/personnels [post]
func (h *PersonnelHandler) CreatePersonnel(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	req, err := bindCreatePersonnelRequest(c)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
			Data:    nil,
		})
	}

	// Upload photo if provided as multipart file
	if key, uerr := utils.UploadFormFile(ctx, c, "photo", "PERSONNEL/PHOTOS"); uerr != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to upload photo: %v", uerr),
		})
	} else if key != nil {
		req.Photo = key
	}

	// Tenant clamp: BUJP-scoped callers may only create personnel under
	// their own BUJP, regardless of what they posted.
	if role, _ := c.Locals("role").(string); !utils.IsPusat(role) {
		if scoped, ok := c.Locals("bujpID").(uuid.UUID); ok && scoped != uuid.Nil {
			req.BujpID = scoped
		}
	}

	personnel, err := h.service.Create(ctx, req)
	if err != nil {
		return c.Status(statusForServiceError(err)).JSON(model.APIResponse{
			Status:  "error",
			Code:    statusForServiceError(err),
			Message: err.Error(),
		})
	}

	// If the client uploaded the photo to a temporary location and supplied a
	// templated final path, move the object now that we know the personnel ID
	// and patch the photo column. Failures here are non-fatal: the personnel
	// row is already persisted, so we surface a warning instead of rolling
	// back the create.
	if req.PhotoTempPath != nil && req.PhotoFinalPath != nil && *req.PhotoTempPath != "" && *req.PhotoFinalPath != "" {
		finalKey := strings.ReplaceAll(*req.PhotoFinalPath, "{personnel_id}", personnel.ID.String())
		if merr := utils.MoveFileInS3(ctx, *req.PhotoTempPath, finalKey); merr == nil {
			update := &model.UpdatePersonnelRequest{Photo: &finalKey}
			if updated, uerr := h.service.Update(ctx, personnel.ID, update); uerr == nil {
				personnel = updated
			}
		}
	}

	return c.Status(http.StatusCreated).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusCreated,
		Message: "Personnel created successfully",
		Data:    personnel,
	})
}

// UpdatePersonnel godoc
// @Summary      Update a personnel
// @Description  Update an existing personnel by ID
// @Tags         Personnels
// @Accept       json
// @Produce      json
// @Param        id       path      string                       true  "Personnel UUID"
// @Param        request  body      model.UpdatePersonnelRequest true  "Updated personnel data"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/personnels/{id} [put]
func (h *PersonnelHandler) UpdatePersonnel(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	req, err := bindUpdatePersonnelRequest(c)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid request body: %v", err),
			Data:    nil,
		})
	}

	if key, uerr := utils.UploadFormFile(ctx, c, "photo", "PERSONNEL/PHOTOS"); uerr != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to upload photo: %v", uerr),
		})
	} else if key != nil {
		req.Photo = key
	}

	// Tenant ownership check before update.
	existing, gerr := h.service.GetByID(ctx, id)
	if gerr != nil || existing == nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: "Personnel not found",
		})
	}
	if !utils.CanAccessPersonnel(c, existing.ID, existing.BujpID) {
		return forbiddenResponse(c)
	}
	// Prevent BUJP-scoped callers from re-homing a personnel to another tenant.
	if role, _ := c.Locals("role").(string); !utils.IsPusat(role) {
		req.BujpID = nil
	}

	personnel, err := h.service.Update(ctx, id, req)
	if err != nil {
		return c.Status(statusForServiceError(err)).JSON(model.APIResponse{
			Status:  "error",
			Code:    statusForServiceError(err),
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Personnel updated successfully",
		Data:    personnel,
	})
}

// bindCreatePersonnelRequest reads CreatePersonnelRequest from JSON body or
// multipart/form-data so the admin client can submit the photo file alongside
// the structured fields in a single request.
func bindCreatePersonnelRequest(c *fiber.Ctx) (*model.CreatePersonnelRequest, error) {
	contentType := strings.ToLower(c.Get("Content-Type"))
	if !(strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded")) {
		var req model.CreatePersonnelRequest
		if err := c.BodyParser(&req); err != nil {
			return nil, err
		}
		return &req, nil
	}

	req := &model.CreatePersonnelRequest{
		BujpID:     utils.FormUUID(c, "bujp_id"),
		IDNumber:   utils.FormString(c, "id_number"),
		FullName:   utils.FormString(c, "full_name"),
		Status:     utils.FormString(c, "status"),
		BaseSalary: utils.FormString(c, "base_salary"),
	}
	if uid := utils.FormUUID(c, "user_id"); uid != uuid.Nil {
		req.UserID = &uid
	}
	if v := utils.FormString(c, "mother_maiden_name"); v != "" {
		req.MotherMaidenName = &v
	}
	if v := utils.FormString(c, "photo"); v != "" {
		req.Photo = &v
	}
	if v := utils.FormString(c, "gender"); v != "" {
		req.Gender = &v
	}
	if v := utils.FormString(c, "address"); v != "" {
		req.Address = &v
	}
	if v := utils.FormString(c, "phone"); v != "" {
		req.Phone = &v
	}
	if v := utils.FormString(c, "emergency_phone"); v != "" {
		req.EmergencyPhone = &v
	}
	if v := utils.FormString(c, "email"); v != "" {
		req.Email = &v
	}
	if v := utils.FormString(c, "license_number"); v != "" {
		req.LicenseNumber = &v
	}
	if v := utils.FormString(c, "bank_name"); v != "" {
		req.BankName = &v
	}
	if v := utils.FormString(c, "account_number"); v != "" {
		req.AccountNumber = &v
	}
	if v := utils.FormString(c, "photo_temp_path"); v != "" {
		req.PhotoTempPath = &v
	}
	if v := utils.FormString(c, "photo_final_path"); v != "" {
		req.PhotoFinalPath = &v
	}
	if t, err := utils.ParseFlexibleDate(utils.FormString(c, "birth_date")); err == nil {
		cd := model.CustomDate{Time: t}
		req.BirthDate = &cd
	}
	if t, err := utils.ParseFlexibleDate(utils.FormString(c, "join_date")); err == nil {
		req.JoinDate = model.CustomDate{Time: t}
	}
	if t, err := utils.ParseFlexibleDate(utils.FormString(c, "contract_end_date")); err == nil {
		cd := model.CustomDate{Time: t}
		req.ContractEndDate = &cd
	}
	return req, nil
}

// bindUpdatePersonnelRequest mirrors the create binder but for the partial
// UpdatePersonnelRequest; only fields actually present on the form (or in the
// JSON body) are populated so unknown fields stay at their existing DB values.
func bindUpdatePersonnelRequest(c *fiber.Ctx) (*model.UpdatePersonnelRequest, error) {
	contentType := strings.ToLower(c.Get("Content-Type"))
	if !(strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded")) {
		var req model.UpdatePersonnelRequest
		if err := c.BodyParser(&req); err != nil {
			return nil, err
		}
		return &req, nil
	}

	req := &model.UpdatePersonnelRequest{}
	if bid := utils.FormUUID(c, "bujp_id"); bid != uuid.Nil {
		req.BujpID = &bid
	}
	if uid := utils.FormUUID(c, "user_id"); uid != uuid.Nil {
		req.UserID = &uid
	}
	if v := utils.FormString(c, "id_number"); v != "" {
		req.IDNumber = &v
	}
	if v := utils.FormString(c, "nip"); v != "" {
		req.NIP = &v
	}
	if v := utils.FormString(c, "full_name"); v != "" {
		req.FullName = &v
	}
	if v := utils.FormString(c, "mother_maiden_name"); v != "" {
		req.MotherMaidenName = &v
	}
	if v := utils.FormString(c, "photo"); v != "" {
		req.Photo = &v
	}
	if v := utils.FormString(c, "gender"); v != "" {
		req.Gender = &v
	}
	if v := utils.FormString(c, "address"); v != "" {
		req.Address = &v
	}
	if v := utils.FormString(c, "phone"); v != "" {
		req.Phone = &v
	}
	if v := utils.FormString(c, "emergency_phone"); v != "" {
		req.EmergencyPhone = &v
	}
	if v := utils.FormString(c, "email"); v != "" {
		req.Email = &v
	}
	if v := utils.FormString(c, "license_number"); v != "" {
		req.LicenseNumber = &v
	}
	if v := utils.FormString(c, "status"); v != "" {
		req.Status = &v
	}
	if v := utils.FormString(c, "bank_name"); v != "" {
		req.BankName = &v
	}
	if v := utils.FormString(c, "account_number"); v != "" {
		req.AccountNumber = &v
	}
	if v := utils.FormString(c, "base_salary"); v != "" {
		req.BaseSalary = &v
	}
	if v := utils.FormString(c, "photo_temp_path"); v != "" {
		req.PhotoTempPath = &v
	}
	if v := utils.FormString(c, "photo_final_path"); v != "" {
		req.PhotoFinalPath = &v
	}
	if t, err := utils.ParseFlexibleDate(utils.FormString(c, "birth_date")); err == nil {
		cd := model.CustomDate{Time: t}
		req.BirthDate = &cd
	}
	if t, err := utils.ParseFlexibleDate(utils.FormString(c, "join_date")); err == nil {
		cd := model.CustomDate{Time: t}
		req.JoinDate = &cd
	}
	if t, err := utils.ParseFlexibleDate(utils.FormString(c, "contract_end_date")); err == nil {
		cd := model.CustomDate{Time: t}
		req.ContractEndDate = &cd
	}
	return req, nil
}

// DeletePersonnel godoc
// @Summary      Delete a personnel
// @Description  Delete a personnel by ID
// @Tags         Personnels
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Personnel UUID"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      404  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/personnels/{id} [delete]
func (h *PersonnelHandler) DeletePersonnel(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: constants.ErrInvalidUUID,
		})
	}

	// Tenant ownership check before delete to prevent cross-BUJP destruction.
	existing, gerr := h.service.GetByID(c.Context(), id)
	if gerr != nil || existing == nil {
		return c.Status(http.StatusNotFound).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusNotFound,
			Message: "Personnel not found",
		})
	}
	if !utils.CanAccessPersonnel(c, existing.ID, existing.BujpID) {
		return forbiddenResponse(c)
	}

	if err := h.service.Delete(c.Context(), id); err != nil {
		statusCode := statusForServiceError(err)
		return c.Status(statusCode).JSON(model.APIResponse{
			Status:  "error",
			Code:    statusCode,
			Message: err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Personnel deleted successfully",
	})
}

// DownloadTemplate godoc
// @Summary      Download personnel bulk-import Excel template
// @Description  Returns a pre-formatted .xlsx template that can be filled and submitted to the bulk import endpoint.
// @Tags         Personnels
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Success      200  {file}  binary
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/personnel/template/download [get]
func (h *PersonnelHandler) DownloadTemplate(c *fiber.Ctx) error {
	bytes, filename, err := h.importService.BuildTemplate()
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Gagal membuat template: " + err.Error(),
		})
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Status(http.StatusOK).Send(bytes)
}

// BulkImport godoc
// @Summary      Bulk import personnel from Excel file
// @Description  Parses the uploaded .xlsx and creates personnel records, auto-creating user accounts (role=guard, default password 123456) for unknown emails. Each row is reported back individually.
// @Tags         Personnels
// @Accept       multipart/form-data
// @Produce      json
// @Param        file     formData  file    true   "Excel file (.xlsx, .xls)"
// @Param        bujp_id  formData  string  false  "BUJP ID (optional for super_admin/admin; non-Pusat callers are clamped to their own BUJP)"
// @Success      200  {object}  model.APIResponse
// @Failure      400  {object}  model.APIResponse
// @Failure      403  {object}  model.APIResponse
// @Failure      500  {object}  model.APIResponse
// @Security     BearerAuth
// @Router       /api/v1/personnel/bulk-import [post]
func (h *PersonnelHandler) BulkImport(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 120*time.Second)
	defer cancel()

	// Resolve effective BUJP. Non-Pusat callers are forced to their own BUJP
	// regardless of what they posted in the form (anti-tampering).
	bujpID, err := uuid.Parse(strings.TrimSpace(c.FormValue("bujp_id")))
	role, _ := c.Locals("role").(string)
	if role != "super_admin" && role != "admin" {
		if scoped, ok := c.Locals("bujpID").(uuid.UUID); ok && scoped != uuid.Nil {
			bujpID = scoped
			err = nil
		} else if userID, ok := c.Locals("userID").(uuid.UUID); ok {
			caller, ferr := h.userRepo.FindByID(ctx, userID)
			if ferr == nil && caller != nil && caller.BujpID != nil {
				bujpID = *caller.BujpID
				err = nil
			}
		}
	}
	if err != nil || bujpID == uuid.Nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "bujp_id wajib diisi dengan format UUID yang valid",
		})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "File Excel wajib diunggah pada field 'file'",
		})
	}
	if fileHeader.Size > 10*1024*1024 {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Ukuran file maksimal 10 MB",
		})
	}
	ext := strings.ToLower(strings.TrimPrefix(fileHeader.Filename[strings.LastIndex(fileHeader.Filename, "."):], "."))
	if ext != "xlsx" && ext != "xls" {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusBadRequest,
			Message: "Format file tidak didukung, gunakan .xlsx atau .xls",
		})
	}

	src, err := fileHeader.Open()
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Gagal membuka file: " + err.Error(),
		})
	}
	defer src.Close()

	buf := make([]byte, fileHeader.Size)
	if _, err := io.ReadFull(src, buf); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Gagal membaca isi file: " + err.Error(),
		})
	}

	summary, err := h.importService.BulkImport(ctx, bujpID, buf)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{
			Status:  "error",
			Code:    http.StatusInternalServerError,
			Message: "Gagal mengimport data personnel: " + err.Error(),
		})
	}

	msg := fmt.Sprintf("Import selesai: %d berhasil", summary.SuccessCount)
	if summary.ErrorCount > 0 {
		msg += fmt.Sprintf(", %d gagal", summary.ErrorCount)
	}
	return c.Status(http.StatusOK).JSON(model.APIResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Message: msg,
		Data:    summary,
	})
}
