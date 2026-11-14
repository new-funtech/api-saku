package handlers

import (
	"net/http"
	"strings"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// ExportPersonnels is every person of the company as an Excel file (the same columns the import
// template uses, so an export can be edited and imported again). The company comes from the
// session; central roles name one with ?bujp_id=.
//
// @Router /personnels/export [get]
func (h *PersonnelHandler) ExportPersonnels(c *fiber.Ctx) error {
	company := utils.CallerBujpID(c)
	if utils.IsPusat(utils.CallerRole(c)) {
		company = uuid.Nil
		if v := strings.TrimSpace(c.Query("bujp_id")); v != "" {
			id, err := uuid.Parse(v)
			if err != nil {
				return c.Status(http.StatusBadRequest).JSON(model.APIResponse{Status: "error", Code: http.StatusBadRequest, Message: "bujp_id is invalid"})
			}
			company = id
		}
	}
	if company == uuid.Nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{Status: "error", Code: http.StatusBadRequest, Message: "bujp_id wajib diisi"})
	}

	people, err := h.service.GetByBujpID(c.Context(), company)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{Status: "error", Code: http.StatusInternalServerError, Message: "Gagal memuat data personel"})
	}

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	const sheet = "Personel"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"Nama Lengkap", "NIK", "NIP", "Telepon", "Email", "Jenis Kelamin", "Tanggal Lahir", "Tanggal Bergabung", "Status", "Gaji Pokok", "Bank", "No. Rekening", "Alamat"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return spreadsheetSafe(*p)
	}
	for i, p := range people {
		row := []interface{}{
			spreadsheetSafe(p.FullName), spreadsheetSafe(p.IDNumber), str(p.NIP), str(p.Phone), str(p.Email), str(p.Gender), str(p.BirthDate),
			p.JoinDate, p.Status, p.BaseSalary, str(p.BankName), str(p.AccountNumber), str(p.Address),
		}
		for j, v := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{Status: "error", Code: http.StatusInternalServerError, Message: "Gagal membuat file Excel"})
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="personel.xlsx"`)
	return c.Send(buf.Bytes())
}
