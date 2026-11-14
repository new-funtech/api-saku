package handlers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ganiramadhan/ganipedia/backend/internal/attendanceq"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Asynchronous clock-in/out (ATTENDANCE_ASYNC=true): the request is validated and published, the
// answer is 202 with the idempotency key, and a worker stores it. See docs/attendance-async.md.

// asyncPhoto turns an uploaded file (multipart) or an inline base64 value into a data URL the worker
// uploads later. The photo is never written to storage inside the request in this mode.
func asyncPhoto(c *fiber.Ctx, field string, inline *string, max int) (*string, error) {
	if fh, err := c.FormFile(field); err == nil && fh != nil {
		if int(fh.Size) > max {
			return nil, fmt.Errorf("foto terlalu besar (maksimal %d KB)", max/1024)
		}
		f, err := fh.Open()
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, int64(max)+1))
		if err != nil {
			return nil, err
		}
		ct := fh.Header.Get("Content-Type")
		if ct == "" {
			ct = "image/jpeg"
		}
		v := "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data)
		return &v, nil
	}
	if inline != nil && len(*inline) > max*4/3+64 {
		return nil, fmt.Errorf("foto terlalu besar (maksimal %d KB)", max/1024)
	}
	return inline, nil
}

func (h *AttendanceHandler) accepted(c *fiber.Ctx, a *services.AsyncAccepted) error {
	c.Set("Idempotency-Key", a.IdempotencyKey)
	c.Set("Location", a.StatusURL)
	return c.Status(http.StatusAccepted).JSON(model.APIResponse{
		Status: "accepted", Code: http.StatusAccepted,
		Message: "Absensi diterima dan sedang diproses",
		Data:    a,
	})
}

func (h *AttendanceHandler) fail(c *fiber.Ctx, err error) error {
	status := statusForServiceError(err)
	return c.Status(status).JSON(model.APIResponse{Status: "error", Code: status, Message: err.Error()})
}

func (h *AttendanceHandler) clockInAsync(c *fiber.Ctx, req *model.CreateAttendanceRequest, key string) error {
	ctx := c.UserContext()
	maxPhoto := attendanceq.LoadConfig().MaxPhotoSize
	photo, err := asyncPhoto(c, "check_in_photo", req.CheckInPhoto, maxPhoto)
	if err != nil {
		return h.fail(c, err)
	}
	req.CheckInPhoto, req.IdempotencyKey = photo, key
	personnelID, _ := c.Locals("personnelID").(uuid.UUID)
	acc, err := h.service.ClockInAsync(ctx, personnelID, req)
	if err != nil {
		return h.fail(c, err)
	}
	return h.accepted(c, acc)
}

func (h *AttendanceHandler) clockOutAsync(c *fiber.Ctx, req *model.UpdateAttendanceRequest, key string) error {
	ctx := c.UserContext()
	maxPhoto := attendanceq.LoadConfig().MaxPhotoSize
	photo, err := asyncPhoto(c, "check_out_photo", req.CheckOutPhoto, maxPhoto)
	if err != nil {
		return h.fail(c, err)
	}
	req.CheckOutPhoto, req.IdempotencyKey = photo, key
	personnelID, _ := c.Locals("personnelID").(uuid.UUID)
	acc, err := h.service.ClockOutAsync(ctx, personnelID, req)
	if err != nil {
		return h.fail(c, err)
	}
	return h.accepted(c, acc)
}

// QueueStatus godoc
// @Summary Status of an asynchronous clock-in/out
// @Description pending = queued or not yet stored, recorded = stored, failed = the worker gave up (see error)
// @Tags Attendances
// @Produce json
// @Param key path string true "Idempotency key returned with the 202"
// @Router /attendances/status/{key} [get]
// @Security BearerAuth
func (h *AttendanceHandler) QueueStatus(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Params("key"))
	if _, err := uuid.Parse(key); err != nil {
		return h.fail(c, errors.New("invalid idempotency key"))
	}
	personnelID, _ := c.Locals("personnelID").(uuid.UUID)
	if personnelID == uuid.Nil {
		return h.fail(c, errors.New("personnel record for current user not found"))
	}
	st, err := h.service.QueueStatus(c.UserContext(), personnelID, key)
	if err != nil {
		return h.fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(model.APIResponse{Status: "success", Code: http.StatusOK, Message: "OK", Data: st})
}
