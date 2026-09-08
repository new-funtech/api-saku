package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/google/uuid"
)

func presignAttendancePhotos(ctx context.Context, r *model.AttendanceResponse) {
	if r == nil {
		return
	}
	if r.CheckInPhoto != nil && *r.CheckInPhoto != "" {
		if u, err := utils.GeneratePresignedURL(ctx, *r.CheckInPhoto, time.Hour); err == nil && u != "" {
			copyU := u
			r.CheckInPhotoURL = &copyU
		}
	}
	if r.CheckOutPhoto != nil && *r.CheckOutPhoto != "" {
		if u, err := utils.GeneratePresignedURL(ctx, *r.CheckOutPhoto, time.Hour); err == nil && u != "" {
			copyU := u
			r.CheckOutPhotoURL = &copyU
		}
	}
	if r.SupportingDocument != nil && *r.SupportingDocument != "" {
		if u, err := utils.GeneratePresignedURL(ctx, *r.SupportingDocument, time.Hour); err == nil && u != "" {
			copyU := u
			r.SupportingDocumentURL = &copyU
		}
	}
}

type AttendanceService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.AttendanceResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.AttendanceResponse, error)
	GetByPersonnelAndDate(ctx context.Context, personnelID uuid.UUID, date time.Time) (*model.AttendanceResponse, error)
	GetByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.AttendanceResponse, int64, error)
	GetByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.AttendanceResponse, int64, error)
	GetByDateRange(ctx context.Context, startDate, endDate time.Time, page, limit int) ([]model.AttendanceResponse, int64, error)
	Create(ctx context.Context, req *model.CreateAttendanceRequest) (*model.AttendanceResponse, error)
	CreateForUser(ctx context.Context, userID uuid.UUID, req *model.CreateAttendanceRequest) (*model.AttendanceResponse, error)
	CheckoutForUser(ctx context.Context, userID uuid.UUID, req *model.UpdateAttendanceRequest) (*model.AttendanceResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateAttendanceRequest) (*model.AttendanceResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type attendanceServiceImpl struct {
	repo           repository.AttendanceRepository
	personnelRepo  repository.PersonnelRepository
	assignmentRepo repository.AssignmentRepository
}

func NewAttendanceService(repo repository.AttendanceRepository, personnelRepo repository.PersonnelRepository, assignmentRepo repository.AssignmentRepository) AttendanceService {
	return &attendanceServiceImpl{repo: repo, personnelRepo: personnelRepo, assignmentRepo: assignmentRepo}
}

func (s *attendanceServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.AttendanceResponse, int64, error) {
	attendances, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.AttendanceResponse, len(attendances))
	for i, attendance := range attendances {
		responses[i] = toAttendanceResponse(&attendance)
		presignAttendancePhotos(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *attendanceServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.AttendanceResponse, error) {
	attendance, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toAttendanceResponse(attendance)
	presignAttendancePhotos(ctx, &response)
	return &response, nil
}

func (s *attendanceServiceImpl) GetByPersonnelAndDate(ctx context.Context, personnelID uuid.UUID, date time.Time) (*model.AttendanceResponse, error) {
	attendance, err := s.repo.FindByPersonnelAndDate(ctx, personnelID, date)
	if err != nil {
		return nil, err
	}

	response := toAttendanceResponse(attendance)
	presignAttendancePhotos(ctx, &response)
	return &response, nil
}

func (s *attendanceServiceImpl) GetByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.AttendanceResponse, int64, error) {
	attendances, total, err := s.repo.FindByPersonnelID(ctx, personnelID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.AttendanceResponse, len(attendances))
	for i, attendance := range attendances {
		responses[i] = toAttendanceResponse(&attendance)
		presignAttendancePhotos(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *attendanceServiceImpl) GetByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.AttendanceResponse, int64, error) {
	attendances, total, err := s.repo.FindByLocationID(ctx, locationID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.AttendanceResponse, len(attendances))
	for i, attendance := range attendances {
		responses[i] = toAttendanceResponse(&attendance)
		presignAttendancePhotos(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *attendanceServiceImpl) GetByDateRange(ctx context.Context, startDate, endDate time.Time, page, limit int) ([]model.AttendanceResponse, int64, error) {
	attendances, total, err := s.repo.FindByDateRange(ctx, startDate, endDate, page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.AttendanceResponse, len(attendances))
	for i, attendance := range attendances {
		responses[i] = toAttendanceResponse(&attendance)
		presignAttendancePhotos(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *attendanceServiceImpl) Create(ctx context.Context, req *model.CreateAttendanceRequest) (*model.AttendanceResponse, error) {
	// Parse date
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, errors.New("invalid date format, expected YYYY-MM-DD")
	}

	// Check for existing attendance for this personnel on this date (unique constraint)
	existing, err := s.repo.FindByPersonnelAndDate(ctx, req.PersonnelID, date)
	if err == nil && existing != nil {
		return nil, errors.New("attendance record already exists for this personnel on this date")
	}

	// Parse check-in time if provided
	var checkIn *time.Time
	if req.CheckIn != nil && *req.CheckIn != "" {
		parsed, err := time.Parse(time.RFC3339, *req.CheckIn)
		if err != nil {
			return nil, errors.New("invalid check_in format, expected RFC3339")
		}
		checkIn = &parsed
	}

	// Parse check-out time if provided
	var checkOut *time.Time
	if req.CheckOut != nil && *req.CheckOut != "" {
		parsed, err := time.Parse(time.RFC3339, *req.CheckOut)
		if err != nil {
			return nil, errors.New("invalid check_out format, expected RFC3339")
		}
		checkOut = &parsed
	}

	// Calculate work duration if both check-in and check-out exist
	var workDuration *int
	if checkIn != nil && checkOut != nil {
		duration := int(checkOut.Sub(*checkIn).Minutes())
		if duration < 0 {
			return nil, errors.New("check_out cannot be before check_in")
		}
		workDuration = &duration
	}

	// Set default status if not provided
	status := req.Status
	if status == "" {
		status = "present"
	}

	attendance := &model.Attendance{
		PersonnelID:        req.PersonnelID,
		LocationID:         req.LocationID,
		AssignmentID:       req.AssignmentID,
		Date:               date,
		CheckIn:            checkIn,
		CheckOut:           checkOut,
		CheckInLatitude:    req.CheckInLatitude.PtrString(),
		CheckInLongitude:   req.CheckInLongitude.PtrString(),
		CheckOutLatitude:   req.CheckOutLatitude.PtrString(),
		CheckOutLongitude:  req.CheckOutLongitude.PtrString(),
		CheckInPhoto:       req.CheckInPhoto,
		CheckOutPhoto:      req.CheckOutPhoto,
		SupportingDocument: req.SupportingDocument,
		Status:             status,
		Notes:              req.Notes,
		WorkDuration:       workDuration,
	}

	if err := s.repo.Create(ctx, attendance); err != nil {
		return nil, err
	}

	// Fetch created attendance with relations
	created, err := s.repo.FindByID(ctx, attendance.ID)
	if err != nil {
		return nil, err
	}

	response := toAttendanceResponse(created)
	presignAttendancePhotos(ctx, &response)
	return &response, nil
}

func (s *attendanceServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdateAttendanceRequest) (*model.AttendanceResponse, error) {
	// Find existing attendance
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Parse check-out time if provided
	if req.CheckOut != nil && *req.CheckOut != "" {
		parsed, err := time.Parse(time.RFC3339, *req.CheckOut)
		if err != nil {
			return nil, errors.New("invalid check_out format, expected RFC3339")
		}
		existing.CheckOut = &parsed
	}

	// Recalculate work duration if both check-in and check-out exist
	if existing.CheckIn != nil && existing.CheckOut != nil {
		duration := int(existing.CheckOut.Sub(*existing.CheckIn).Minutes())
		if duration < 0 {
			return nil, errors.New("check_out cannot be before check_in")
		}
		existing.WorkDuration = &duration
	}

	// Update fields
	if v := req.CheckOutLatitude.PtrString(); v != nil {
		existing.CheckOutLatitude = v
	}
	if v := req.CheckOutLongitude.PtrString(); v != nil {
		existing.CheckOutLongitude = v
	}
	if req.CheckOutPhoto != nil {
		existing.CheckOutPhoto = req.CheckOutPhoto
	}
	if req.SupportingDocument != nil {
		existing.SupportingDocument = req.SupportingDocument
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}
	if req.Notes != nil {
		existing.Notes = req.Notes
	}
	if req.WorkDuration != nil {
		existing.WorkDuration = req.WorkDuration
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	// Fetch updated attendance with relations
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toAttendanceResponse(updated)
	presignAttendancePhotos(ctx, &response)
	return &response, nil
}

func (s *attendanceServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *attendanceServiceImpl) CreateForUser(ctx context.Context, userID uuid.UUID, req *model.CreateAttendanceRequest) (*model.AttendanceResponse, error) {
	if userID == uuid.Nil {
		return nil, errors.New("unauthenticated")
	}
	personnel, err := s.personnelRepo.FindByUserID(ctx, userID)
	if err != nil || personnel == nil {
		return nil, errors.New("personnel record for current user not found")
	}
	req.PersonnelID = personnel.ID

	if req.AssignmentID == nil || *req.AssignmentID == uuid.Nil || req.LocationID == uuid.Nil {
		assignment, aerr := s.assignmentRepo.FindActiveByPersonnelID(ctx, personnel.ID)
		if aerr != nil || assignment == nil {
			return nil, errors.New("no active assignment for current user")
		}
		if req.AssignmentID == nil || *req.AssignmentID == uuid.Nil {
			id := assignment.ID
			req.AssignmentID = &id
		}
		if req.LocationID == uuid.Nil {
			req.LocationID = assignment.LocationID
		}
	}

	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}
	if req.CheckIn == nil || *req.CheckIn == "" {
		now := time.Now().Format(time.RFC3339)
		req.CheckIn = &now
	}
	return s.Create(ctx, req)
}

func (s *attendanceServiceImpl) CheckoutForUser(ctx context.Context, userID uuid.UUID, req *model.UpdateAttendanceRequest) (*model.AttendanceResponse, error) {
	if userID == uuid.Nil {
		return nil, errors.New("unauthenticated")
	}
	personnel, err := s.personnelRepo.FindByUserID(ctx, userID)
	if err != nil || personnel == nil {
		return nil, errors.New("personnel record for current user not found")
	}
	today := time.Now().Truncate(24 * time.Hour)
	att, err := s.repo.FindByPersonnelAndDate(ctx, personnel.ID, today)
	if err != nil || att == nil {
		return nil, errors.New("no attendance record for today; please check-in first")
	}

	checkOutTime := time.Now()
	if req.CheckOut != nil && *req.CheckOut != "" {
		if parsed, perr := time.Parse(time.RFC3339, *req.CheckOut); perr == nil {
			checkOutTime = parsed
		}
	}

	if att.Assignment != nil && att.Assignment.Shift != nil {
		shiftEnd := att.Assignment.Shift.EndTime.Time
		shiftStart := att.Assignment.Shift.StartTime.Time
		if !shiftEnd.IsZero() {
			end := time.Date(
				att.Date.Year(), att.Date.Month(), att.Date.Day(),
				shiftEnd.Hour(), shiftEnd.Minute(), shiftEnd.Second(), 0,
				checkOutTime.Location(),
			)
			if !shiftStart.IsZero() && !shiftEnd.After(shiftStart) {
				end = end.Add(24 * time.Hour)
			}
			if checkOutTime.Before(end) {
				return nil, fmt.Errorf(
					"belum waktunya checkout, shift Anda berakhir pukul %s",
					shiftEnd.Format("15:04"),
				)
			}
		}
	}

	if req.CheckOut == nil || *req.CheckOut == "" {
		now := checkOutTime.Format(time.RFC3339)
		req.CheckOut = &now
	}
	return s.Update(ctx, att.ID, req)
}

// Helper functions

func toAttendanceResponse(attendance *model.Attendance) model.AttendanceResponse {
	response := model.AttendanceResponse{
		ID:                 attendance.ID,
		PersonnelID:        attendance.PersonnelID,
		LocationID:         attendance.LocationID,
		AssignmentID:       attendance.AssignmentID,
		Date:               attendance.Date.Format("2006-01-02"),
		CheckInLatitude:    attendance.CheckInLatitude,
		CheckInLongitude:   attendance.CheckInLongitude,
		CheckOutLatitude:   attendance.CheckOutLatitude,
		CheckOutLongitude:  attendance.CheckOutLongitude,
		CheckInPhoto:       attendance.CheckInPhoto,
		CheckOutPhoto:      attendance.CheckOutPhoto,
		SupportingDocument: attendance.SupportingDocument,
		Status:             attendance.Status,
		Notes:              attendance.Notes,
		WorkDuration:       attendance.WorkDuration,
		CreatedAt:          attendance.CreatedAt,
		UpdatedAt:          attendance.UpdatedAt,
	}

	if attendance.CheckIn != nil {
		checkIn := attendance.CheckIn.Format(time.RFC3339)
		response.CheckIn = &checkIn
	}

	if attendance.CheckOut != nil {
		checkOut := attendance.CheckOut.Format(time.RFC3339)
		response.CheckOut = &checkOut
	}

	if attendance.Personnel != nil {
		personnelResponse := ToPersonnelResponse(attendance.Personnel)
		response.Personnel = &personnelResponse
	}

	if attendance.Location != nil {
		locationResponse := model.LocationResponse{
			ID:               attendance.Location.ID,
			BujpID:           attendance.Location.BujpID,
			Code:             attendance.Location.Code,
			Name:             attendance.Location.Name,
			Address:          attendance.Location.Address,
			Latitude:         attendance.Location.Latitude,
			Longitude:        attendance.Location.Longitude,
			AttendanceRadius: attendance.Location.AttendanceRadius,
			GuardsNeeded:     attendance.Location.GuardsNeeded,
			PicName:          attendance.Location.PicName,
			PicPhone:         attendance.Location.PicPhone,
			Status:           attendance.Location.Status,
			CreatedAt:        attendance.Location.CreatedAt,
			UpdatedAt:        attendance.Location.UpdatedAt,
		}
		response.Location = &locationResponse
	}

	if attendance.Assignment != nil {
		assignmentResponse := model.AssignmentResponse{
			ID:          attendance.Assignment.ID,
			PersonnelID: attendance.Assignment.PersonnelID,
			LocationID:  attendance.Assignment.LocationID,
			ShiftID:     attendance.Assignment.ShiftID,
			StartDate:   attendance.Assignment.StartDate.Format("2006-01-02"),
			Status:      attendance.Assignment.Status,
			Notes:       attendance.Assignment.Notes,
			CreatedBy:   attendance.Assignment.CreatedBy,
			CreatedAt:   attendance.Assignment.CreatedAt,
			UpdatedAt:   attendance.Assignment.UpdatedAt,
		}
		if attendance.Assignment.EndDate != nil {
			endDate := attendance.Assignment.EndDate.Format("2006-01-02")
			assignmentResponse.EndDate = &endDate
		}
		response.Assignment = &assignmentResponse
	}

	return response
}
