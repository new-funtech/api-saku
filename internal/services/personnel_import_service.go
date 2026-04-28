package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Default password assigned to users that the personnel importer auto-creates
// when an Excel row references an email that does not yet exist. Mirrors the
// Laravel implementation (`123456`).
const PersonnelImportDefaultPassword = "123456"

// PersonnelImportRowResult is a single successful import outcome.
type PersonnelImportRowResult struct {
	Row         int       `json:"row"`
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	FullName    string    `json:"full_name"`
	UserCreated bool      `json:"user_created"`
}

// PersonnelImportRowError is a single failed row with the offending value(s).
type PersonnelImportRowError struct {
	Row     int                    `json:"row"`
	Field   string                 `json:"field"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// PersonnelImportSummary is the response payload returned by BulkImport.
type PersonnelImportSummary struct {
	SuccessCount int                        `json:"success_count"`
	ErrorCount   int                        `json:"error_count"`
	Results      []PersonnelImportRowResult `json:"results"`
	Errors       []PersonnelImportRowError  `json:"errors"`
}

// PersonnelImportService exposes the bulk-import flow used by the admin UI.
type PersonnelImportService interface {
	BulkImport(ctx context.Context, bujpID uuid.UUID, fileBytes []byte) (*PersonnelImportSummary, error)
	BuildTemplate() ([]byte, string, error)
}

type personnelImportServiceImpl struct {
	personnelRepo repository.PersonnelRepository
	userRepo      repository.UserRepository
}

// NewPersonnelImportService wires the bulk import service.
func NewPersonnelImportService(personnelRepo repository.PersonnelRepository, userRepo repository.UserRepository) PersonnelImportService {
	return &personnelImportServiceImpl{personnelRepo: personnelRepo, userRepo: userRepo}
}

// templateHeaders defines the order/labels of columns in the downloadable
// template. Must stay aligned with the field names parsed by BulkImport.
var templateHeaders = []string{
	"email", "full_name", "id_number", "nip", "phone", "mother_maiden_name",
	"emergency_phone", "address", "gender", "birth_date", "license_number",
	"bank_name", "account_number", "join_date", "contract_end_date",
	"base_salary", "status",
}

// BuildTemplate generates the .xlsx template a user should fill before bulk
// importing. Returns the raw bytes, the suggested file name, and an error.
func (s *personnelImportServiceImpl) BuildTemplate() ([]byte, string, error) {
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "Personnel Template"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, "", err
	}
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return nil, "", err
	}
	f.SetActiveSheet(idx)

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	for i, h := range templateHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	lastCol, _ := excelize.ColumnNumberToName(len(templateHeaders))
	_ = f.SetCellStyle(sheet, "A1", lastCol+"1", headerStyle)

	// Sample placeholder row so the user immediately sees the expected format.
	sample := []interface{}{
		"email@example.com", "Nama Lengkap", "3212113010030001", "19850101201001001", "081234567890",
		"Nama Ibu Kandung", "081234567891", "Alamat Lengkap", "M",
		"1990-01-15", "LIC001", "Bank BJB", "1234567890",
		time.Now().Format("2006-01-02"), time.Now().AddDate(1, 0, 0).Format("2006-01-02"),
		"5000000", "active",
	}
	for i, v := range sample {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		_ = f.SetCellValue(sheet, cell, v)
	}

	// Auto-size columns to a reasonable width.
	for i := range templateHeaders {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheet, col, col, 22)
	}

	// Notes sheet.
	const notes = "Petunjuk"
	if _, err := f.NewSheet(notes); err != nil {
		return nil, "", err
	}
	titleStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}})
	warnStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FF0000"}})
	rows := [][]interface{}{
		{"PETUNJUK PENGISIAN DATA PERSONNEL"},
		{"PENTING - Baca Dulu Sebelum Mengisi:"},
		{"", "1. Hapus baris contoh di sheet Personnel Template (baris 2)"},
		{"", "2. Email wajib diisi - sistem otomatis membuat user baru jika belum ada"},
		{"", "3. Password default untuk user baru: " + PersonnelImportDefaultPassword},
		{"", "4. NIK (Nomor KTP) harus 16 digit angka"},
		{"", "5. Mulai isi data dari baris ke-3 di sheet Personnel Template"},
		{},
		{"Kolom", "Keterangan", "Contoh", "Wajib"},
		{"email", "Email (otomatis buat user baru jika belum ada)", "john@email.com", "Ya"},
		{"full_name", "Nama lengkap", "JOHN DOE", "Ya"},
		{"id_number", "NIK - Nomor KTP (16 digit)", "3212113010030001", "Ya"},
		{"nip", "NIP - Nomor Induk Pegawai", "19850101201001001", "Ya"},
		{"phone", "Nomor HP", "081234567890", "Tidak"},
		{"mother_maiden_name", "Nama ibu kandung", "JANE DOE", "Tidak"},
		{"emergency_phone", "Nomor HP darurat", "081234567891", "Tidak"},
		{"address", "Alamat lengkap", "Jl. Contoh No. 1", "Tidak"},
		{"gender", "Jenis kelamin (M/F)", "M", "Tidak"},
		{"birth_date", "Tanggal lahir (YYYY-MM-DD)", "1990-01-15", "Tidak"},
		{"license_number", "Nomor lisensi", "LIC001", "Tidak"},
		{"bank_name", "Nama bank", "Bank BJB", "Tidak"},
		{"account_number", "Nomor rekening", "1234567890", "Tidak"},
		{"join_date", "Tanggal mulai bekerja (YYYY-MM-DD)", "2026-01-01", "Ya"},
		{"contract_end_date", "Tanggal akhir kontrak (YYYY-MM-DD)", "2027-01-01", "Tidak"},
		{"base_salary", "Gaji pokok (angka saja)", "5000000", "Tidak"},
		{"status", "Status (active/inactive/on_leave/resigned)", "active", "Tidak"},
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			_ = f.SetCellValue(notes, cell, v)
		}
	}
	_ = f.SetCellStyle(notes, "A1", "A1", titleStyle)
	_ = f.SetCellStyle(notes, "A2", "A2", warnStyle)
	_ = f.SetCellStyle(notes, "A9", "D9", headerStyle)
	for _, col := range []string{"A", "B", "C", "D"} {
		_ = f.SetColWidth(notes, col, col, 32)
	}

	f.SetActiveSheet(idx)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, "", err
	}
	filename := fmt.Sprintf("template_personnel_import_%s.xlsx", time.Now().Format("20060102_150405"))
	return buf.Bytes(), filename, nil
}

// BulkImport parses an .xlsx file and creates Personnel records (auto-creating
// User accounts when needed). Each row is processed independently — a row
// failure does not abort the rest of the import.
func (s *personnelImportServiceImpl) BulkImport(ctx context.Context, bujpID uuid.UUID, fileBytes []byte) (*PersonnelImportSummary, error) {
	if bujpID == uuid.Nil {
		return nil, errors.New("bujp_id wajib diisi")
	}

	f, err := excelize.OpenReader(bytes.NewReader(fileBytes))
	if err != nil {
		return nil, fmt.Errorf("file tidak dapat dibaca: %w", err)
	}
	defer f.Close()

	// Use the first sheet — matches Laravel which reads via heading row on the
	// active sheet of the uploaded workbook.
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, errors.New("file Excel tidak memiliki sheet")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca baris Excel: %w", err)
	}
	if len(rows) < 2 {
		return nil, errors.New("file Excel kosong atau hanya berisi header")
	}

	headers := normalizeHeaders(rows[0])
	hashedDefaultPassword, err := bcrypt.GenerateFromPassword([]byte(PersonnelImportDefaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	summary := &PersonnelImportSummary{Results: []PersonnelImportRowResult{}, Errors: []PersonnelImportRowError{}}

	for i := 1; i < len(rows); i++ {
		rowNumber := i + 1
		row := toRowMap(headers, rows[i])

		// Skip wholly-empty rows.
		if isEmptyImportRow(row) {
			continue
		}

		email := strings.TrimSpace(row["email"])
		fullName := strings.TrimSpace(row["full_name"])
		idNumber := strings.TrimSpace(row["id_number"])

		if email == "" {
			summary.Errors = append(summary.Errors, PersonnelImportRowError{Row: rowNumber, Field: "email", Message: "Email wajib diisi", Data: stringMapToAny(row)})
			continue
		}
		if !isValidEmail(email) {
			summary.Errors = append(summary.Errors, PersonnelImportRowError{Row: rowNumber, Field: "email", Message: fmt.Sprintf("Format email '%s' tidak valid", email), Data: stringMapToAny(row)})
			continue
		}
		if fullName == "" {
			summary.Errors = append(summary.Errors, PersonnelImportRowError{Row: rowNumber, Field: "full_name", Message: "Nama lengkap wajib diisi", Data: stringMapToAny(row)})
			continue
		}
		if idNumber == "" {
			summary.Errors = append(summary.Errors, PersonnelImportRowError{Row: rowNumber, Field: "id_number", Message: "Nomor identitas (KTP) wajib diisi", Data: stringMapToAny(row)})
			continue
		}
		nipValue := strings.TrimSpace(row["nip"])
		if nipValue == "" {
			summary.Errors = append(summary.Errors, PersonnelImportRowError{Row: rowNumber, Field: "nip", Message: "NIP wajib diisi", Data: stringMapToAny(row)})
			continue
		}

		// Find or auto-create the user.
		user, ferr := s.userRepo.FindByEmail(ctx, email)
		if ferr != nil && !errors.Is(ferr, gorm.ErrRecordNotFound) {
			summary.Errors = append(summary.Errors, PersonnelImportRowError{Row: rowNumber, Field: "email", Message: ferr.Error(), Data: stringMapToAny(row)})
			continue
		}

		userCreated := false
		if user == nil {
			phone := strPtrIfNotEmpty(row["phone"])
			user = &model.User{
				BujpID:   &bujpID,
				FullName: fullName,
				Email:    email,
				Password: string(hashedDefaultPassword),
				Phone:    phone,
				Role:     "guard",
				Status:   "active",
			}
			if cerr := s.userRepo.Create(ctx, user); cerr != nil {
				summary.Errors = append(summary.Errors, PersonnelImportRowError{Row: rowNumber, Field: "email", Message: humanizeUserCreateError(cerr, email), Data: stringMapToAny(row)})
				continue
			}
			userCreated = true
		}

		// Reject duplicates: same NIK or same (user, bujp) pair.
		if existing, _ := s.personnelRepo.FindByIDNumber(ctx, idNumber); existing != nil {
			summary.Errors = append(summary.Errors, PersonnelImportRowError{Row: rowNumber, Field: "id_number", Message: fmt.Sprintf("NIK %s sudah terdaftar dalam sistem", idNumber), Data: stringMapToAny(row)})
			continue
		}
		if existing, _ := s.personnelRepo.FindByUserID(ctx, user.ID); existing != nil && existing.BujpID == bujpID {
			summary.Errors = append(summary.Errors, PersonnelImportRowError{Row: rowNumber, Field: "email", Message: fmt.Sprintf("Personnel dengan email %s sudah terdaftar di BUJP ini", email), Data: stringMapToAny(row)})
			continue
		}

		joinDate, err := parseImportDate(row["join_date"])
		if err != nil || joinDate == nil {
			summary.Errors = append(summary.Errors, PersonnelImportRowError{Row: rowNumber, Field: "join_date", Message: "Tanggal bergabung wajib diisi (format YYYY-MM-DD)", Data: stringMapToAny(row)})
			continue
		}
		birthDate, _ := parseImportDate(row["birth_date"])
		contractEndDate, _ := parseImportDate(row["contract_end_date"])

		userID := user.ID
		personnel := &model.Personnel{
			BujpID:           bujpID,
			UserID:           &userID,
			IDNumber:         idNumber,
			NIP:              strPtrIfNotEmpty(row["nip"]),
			FullName:         fullName,
			MotherMaidenName: strPtrIfNotEmpty(row["mother_maiden_name"]),
			BirthDate:        birthDate,
			Gender:           normalizeGender(row["gender"]),
			Address:          strPtrIfNotEmpty(row["address"]),
			Phone:            strPtrIfNotEmpty(row["phone"]),
			EmergencyPhone:   strPtrIfNotEmpty(row["emergency_phone"]),
			Email:            strPtrIfNotEmpty(email),
			LicenseNumber:    strPtrIfNotEmpty(row["license_number"]),
			JoinDate:         *joinDate,
			ContractEndDate:  contractEndDate,
			Status:           normalizeImportStatus(row["status"]),
			BankName:         strPtrIfNotEmpty(row["bank_name"]),
			AccountNumber:    strPtrIfNotEmpty(row["account_number"]),
			BaseSalary:       parseSalary(row["base_salary"]),
		}

		if cerr := s.personnelRepo.Create(ctx, personnel); cerr != nil {
			summary.Errors = append(summary.Errors, PersonnelImportRowError{Row: rowNumber, Field: "general", Message: cerr.Error(), Data: stringMapToAny(row)})
			continue
		}

		summary.Results = append(summary.Results, PersonnelImportRowResult{
			Row: rowNumber, ID: personnel.ID, UserID: user.ID, FullName: personnel.FullName, UserCreated: userCreated,
		})
	}

	summary.SuccessCount = len(summary.Results)
	summary.ErrorCount = len(summary.Errors)
	return summary, nil
}

func normalizeHeaders(raw []string) []string {
	headers := make([]string, len(raw))
	for i, h := range raw {
		headers[i] = strings.ToLower(strings.TrimSpace(h))
	}
	return headers
}

func toRowMap(headers, cells []string) map[string]string {
	row := make(map[string]string, len(headers))
	for i, h := range headers {
		if h == "" {
			continue
		}
		if i < len(cells) {
			row[h] = strings.TrimSpace(cells[i])
		} else {
			row[h] = ""
		}
	}
	return row
}

func isEmptyImportRow(row map[string]string) bool {
	for _, v := range row {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

func stringMapToAny(in map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func strPtrIfNotEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func isValidEmail(s string) bool {
	if !strings.Contains(s, "@") || !strings.Contains(s, ".") {
		return false
	}
	at := strings.Index(s, "@")
	return at > 0 && at < len(s)-3
}

func humanizeUserCreateError(err error, email string) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "duplicate key"), strings.Contains(msg, "unique constraint"):
		return fmt.Sprintf("Email %s sudah terdaftar di sistem", email)
	case strings.Contains(msg, "null value"):
		return "Ada data wajib yang belum diisi. Pastikan email dan nama lengkap terisi"
	default:
		return "Gagal membuat user baru: " + msg
	}
}

func normalizeGender(raw string) *string {
	v := strings.ToUpper(strings.TrimSpace(raw))
	if v == "" {
		return nil
	}
	switch v {
	case "M", "L", "LAKI-LAKI", "LAKI", "MALE", "PRIA":
		s := "M"
		return &s
	case "F", "P", "PEREMPUAN", "FEMALE", "WANITA":
		s := "F"
		return &s
	}
	return nil
}

func normalizeImportStatus(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "active"
	}
	switch v {
	case "active", "inactive", "on_leave", "resigned":
		return v
	case "aktif":
		return "active"
	case "tidak aktif", "nonaktif":
		return "inactive"
	case "cuti":
		return "on_leave"
	case "resign", "keluar":
		return "resigned"
	}
	return "active"
}

func parseImportDate(raw string) (*time.Time, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil, nil
	}
	// Excel may give a serial number for date cells.
	if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
		t, err := excelize.ExcelDateToTime(n, false)
		if err == nil {
			return &t, nil
		}
	}
	formats := []string{"2006-01-02", "02-01-2006", "02/01/2006", "2006/01/02", "2006-01-02T15:04:05Z07:00", time.RFC3339}
	for _, f := range formats {
		if t, err := time.Parse(f, v); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("invalid date: %s", v)
}

func parseSalary(raw string) float64 {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0
	}
	// Strip currency formatting (Rp, ., ,, spaces).
	cleaned := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return -1
	}, v)
	if cleaned == "" {
		return 0
	}
	n, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0
	}
	return n
}
