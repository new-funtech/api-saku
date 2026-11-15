package handlers

import (
	"context"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

type loanExportServiceStub struct {
	services.LoanService
	loans []model.LoanResponse
}

func (s loanExportServiceStub) GetAll(
	_ context.Context,
	_ int,
	_ int,
	_ map[string]interface{},
) ([]model.LoanResponse, int64, error) {
	return s.loans, int64(len(s.loans)), nil
}

func TestExportBPRKSReturnsExcelAndRejectsOtherRoles(t *testing.T) {
	nip := "19850101201001001"
	bank := "Bank BJB"
	account := "001234567890"
	approvedAmount := 4500000.0
	handler := NewLoanHandler(loanExportServiceStub{
		loans: []model.LoanResponse{{
			LoanNumber:         "LN-2026-001",
			Status:             model.LoanStatusActive,
			LoanAmount:         5000000,
			ApprovedAmount:     &approvedAmount,
			InterestRate:       1.5,
			TenorMonths:        3,
			MonthlyInstallment: 1550000,
			TotalRepayment:     4650000,
			Personnel: &model.PersonnelResponse{
				NIP:           &nip,
				IDNumber:      "3201234567890123",
				FullName:      "Budi Santoso",
				BankName:      &bank,
				AccountNumber: &account,
				BaseSalary:    7000000,
			},
		}},
	})

	app := fiber.New()
	app.Get("/export/:role", func(c *fiber.Ctx) error {
		c.Locals("role", c.Params("role"))
		return handler.ExportBPRKS(c)
	})

	response, err := app.Test(httptest.NewRequest("GET", "/export/bprks", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	if got := response.Header.Get(fiber.HeaderContentType); got != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("unexpected content type %q", got)
	}

	workbook, err := excelize.OpenReader(response.Body)
	if err != nil {
		t.Fatalf("open exported workbook: %v", err)
	}
	defer workbook.Close()
	for cell, want := range map[string]string{
		"D2": "Budi Santoso",
		"E2": "001234567890",
		"G2": "LN-2026-001",
		"I2": "4500000",
	} {
		got, err := workbook.GetCellValue("Kredit BPRKS", cell)
		if err != nil {
			t.Fatalf("read %s: %v", cell, err)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", cell, got, want)
		}
	}

	denied, err := app.Test(httptest.NewRequest("GET", "/export/guard", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Body.Close()
	if denied.StatusCode != 403 {
		t.Errorf("guard export status = %d, want 403", denied.StatusCode)
	}
	body, err := io.ReadAll(denied.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 || body[0] != '{' {
		t.Error("expected a JSON error response for a denied export")
	}
}
