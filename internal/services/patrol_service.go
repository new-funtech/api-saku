package services

import (
	"context"
	"errors"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/google/uuid"
)

// parsePatrolTime accepts several common datetime formats sent by the various
// clients (mobile, web, admin) and returns a UTC time.Time.
func parsePatrolTime(value string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unsupported patrol_time format")
}

type PatrolService interface {
	GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.PatrolResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.PatrolResponse, error)
	GetByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.PatrolResponse, int64, error)
	GetByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.PatrolResponse, int64, error)
	GetByAttendanceID(ctx context.Context, attendanceID uuid.UUID) ([]model.PatrolResponse, error)
	GetPending(ctx context.Context, page, limit int) ([]model.PatrolResponse, int64, error)
	Create(ctx context.Context, req *model.CreatePatrolRequest) (*model.PatrolResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdatePatrolRequest) (*model.PatrolResponse, error)
	Validate(ctx context.Context, id uuid.UUID, validatorID uuid.UUID, status string) (*model.PatrolResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type patrolServiceImpl struct {
	repo           repository.PatrolRepository
	attendanceRepo repository.AttendanceRepository
}

func NewPatrolService(repo repository.PatrolRepository, attendanceRepo repository.AttendanceRepository) PatrolService {
	return &patrolServiceImpl{repo: repo, attendanceRepo: attendanceRepo}
}

func (s *patrolServiceImpl) GetAll(ctx context.Context, page, limit int, filters map[string]interface{}) ([]model.PatrolResponse, int64, error) {
	patrols, total, err := s.repo.FindAll(ctx, page, limit, filters)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.PatrolResponse, len(patrols))
	for i, patrol := range patrols {
		responses[i] = toPatrolResponse(&patrol)
		presignPatrolPhoto(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *patrolServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*model.PatrolResponse, error) {
	patrol, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toPatrolResponse(patrol)
	presignPatrolPhoto(ctx, &response)
	return &response, nil
}

func (s *patrolServiceImpl) GetByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.PatrolResponse, int64, error) {
	patrols, total, err := s.repo.FindByPersonnelID(ctx, personnelID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.PatrolResponse, len(patrols))
	for i, patrol := range patrols {
		responses[i] = toPatrolResponse(&patrol)
		presignPatrolPhoto(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *patrolServiceImpl) GetByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.PatrolResponse, int64, error) {
	patrols, total, err := s.repo.FindByLocationID(ctx, locationID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.PatrolResponse, len(patrols))
	for i, patrol := range patrols {
		responses[i] = toPatrolResponse(&patrol)
		presignPatrolPhoto(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *patrolServiceImpl) GetByAttendanceID(ctx context.Context, attendanceID uuid.UUID) ([]model.PatrolResponse, error) {
	patrols, err := s.repo.FindByAttendanceID(ctx, attendanceID)
	if err != nil {
		return nil, err
	}

	responses := make([]model.PatrolResponse, len(patrols))
	for i, patrol := range patrols {
		responses[i] = toPatrolResponse(&patrol)
		presignPatrolPhoto(ctx, &responses[i])
	}

	return responses, nil
}

func (s *patrolServiceImpl) GetPending(ctx context.Context, page, limit int) ([]model.PatrolResponse, int64, error) {
	patrols, total, err := s.repo.FindPending(ctx, page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]model.PatrolResponse, len(patrols))
	for i, patrol := range patrols {
		responses[i] = toPatrolResponse(&patrol)
		presignPatrolPhoto(ctx, &responses[i])
	}

	return responses, total, nil
}

func (s *patrolServiceImpl) Create(ctx context.Context, req *model.CreatePatrolRequest) (*model.PatrolResponse, error) {
	// Parse date (accepts YYYY-MM-DD or full timestamps)
	date, err := utils.ParseFlexibleDate(req.Date)
	if err != nil {
		return nil, errors.New("invalid date format, expected YYYY-MM-DD")
	}

	// Parse patrol time. Accept RFC3339 and common alternates such as the MySQL
	// datetime format ("2006-01-02 15:04:05") emitted by the web client.
	patrolTime, err := parsePatrolTime(req.PatrolTime)
	if err != nil {
		return nil, errors.New("invalid patrol_time format")
	}

	// Validate patrol time is on the same date
	if patrolTime.Format("2006-01-02") != date.Format("2006-01-02") {
		return nil, errors.New("patrol_time must be on the same date as date field")
	}

	patrol := &model.Patrol{
		PersonnelID:      req.PersonnelID,
		LocationID:       req.LocationID,
		AttendanceID:     req.AttendanceID,
		Date:             date,
		PatrolTime:       patrolTime,
		PatrolArea:       req.PatrolArea,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		Photo:            req.Photo,
		Notes:            req.Notes,
		ValidationStatus: "pending", // Default status
	}

	// Auto-resolve attendance_id from personnel + date when client did not
	// supply it (mobile/web do not always send it). Patrols and the day's
	// attendance share the same (personnel, date) key.
	if patrol.AttendanceID == nil && s.attendanceRepo != nil {
		if att, err := s.attendanceRepo.FindByPersonnelAndDate(ctx, req.PersonnelID, date); err == nil && att != nil {
			id := att.ID
			patrol.AttendanceID = &id
		}
	}

	if err := s.repo.Create(ctx, patrol); err != nil {
		return nil, err
	}

	// Fetch created patrol with relations
	created, err := s.repo.FindByID(ctx, patrol.ID)
	if err != nil {
		return nil, err
	}

	response := toPatrolResponse(created)
	presignPatrolPhoto(ctx, &response)
	return &response, nil
}

func (s *patrolServiceImpl) Update(ctx context.Context, id uuid.UUID, req *model.UpdatePatrolRequest) (*model.PatrolResponse, error) {
	// Find existing patrol
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Only allow update if status is pending
	if existing.ValidationStatus != "pending" {
		return nil, errors.New("can only update pending patrol records")
	}

	// Update fields
	if req.PatrolArea != nil {
		existing.PatrolArea = req.PatrolArea
	}
	if req.Latitude != nil {
		existing.Latitude = req.Latitude
	}
	if req.Longitude != nil {
		existing.Longitude = req.Longitude
	}
	if req.Photo != nil {
		existing.Photo = req.Photo
	}
	if req.Notes != nil {
		existing.Notes = req.Notes
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	// Fetch updated patrol with relations
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toPatrolResponse(updated)
	presignPatrolPhoto(ctx, &response)
	return &response, nil
}

func (s *patrolServiceImpl) Validate(ctx context.Context, id uuid.UUID, validatorID uuid.UUID, status string) (*model.PatrolResponse, error) {
	// Validate status — accept FE-friendly aliases.
	switch status {
	case "approved", "validated":
		status = "approved"
	case "rejected":
		// ok
	default:
		return nil, errors.New("status must be either 'approved' or 'rejected'")
	}

	// Find existing patrol
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Only allow validation if status is pending
	if existing.ValidationStatus != "pending" {
		return nil, errors.New("can only validate pending patrol records")
	}

	// Validate the patrol
	if err := s.repo.Validate(ctx, id, validatorID, status); err != nil {
		return nil, err
	}

	// Fetch updated patrol with relations
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toPatrolResponse(updated)
	presignPatrolPhoto(ctx, &response)
	return &response, nil
}

func (s *patrolServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	// Find existing patrol
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Only allow deletion if status is pending or rejected
	if existing.ValidationStatus == "approved" {
		return errors.New("cannot delete approved patrol records")
	}

	return s.repo.Delete(ctx, id)
}

// Helper functions

// presignPatrolPhoto fills PhotoURL with a presigned GET URL for the stored
// patrol photo so the client can render it without exposing raw S3 keys.
func presignPatrolPhoto(ctx context.Context, r *model.PatrolResponse) {
	if r == nil || r.Photo == nil || *r.Photo == "" {
		return
	}
	if url, err := utils.GeneratePresignedURL(ctx, *r.Photo, time.Hour); err == nil && url != "" {
		u := url
		r.PhotoURL = &u
	}
}

func toPatrolResponse(patrol *model.Patrol) model.PatrolResponse {
	response := model.PatrolResponse{
		ID:               patrol.ID,
		PersonnelID:      patrol.PersonnelID,
		LocationID:       patrol.LocationID,
		AttendanceID:     patrol.AttendanceID,
		Date:             patrol.Date.Format("2006-01-02"),
		PatrolTime:       patrol.PatrolTime.Format(time.RFC3339),
		PatrolArea:       patrol.PatrolArea,
		Latitude:         patrol.Latitude,
		Longitude:        patrol.Longitude,
		Photo:            patrol.Photo,
		Notes:            patrol.Notes,
		ValidationStatus: patrol.ValidationStatus,
		ValidatorID:      patrol.ValidatorID,
		CreatedAt:        patrol.CreatedAt,
		UpdatedAt:        patrol.UpdatedAt,
	}

	if patrol.ValidatedAt != nil {
		response.ValidatedAt = patrol.ValidatedAt
	}

	if patrol.Personnel != nil {
		personnelResponse := ToPersonnelResponse(patrol.Personnel)
		response.Personnel = &personnelResponse
	}

	if patrol.Location != nil {
		locationResponse := model.LocationResponse{
			ID:               patrol.Location.ID,
			BujpID:           patrol.Location.BujpID,
			Code:             patrol.Location.Code,
			Name:             patrol.Location.Name,
			Address:          patrol.Location.Address,
			Latitude:         patrol.Location.Latitude,
			Longitude:        patrol.Location.Longitude,
			AttendanceRadius: patrol.Location.AttendanceRadius,
			GuardsNeeded:     patrol.Location.GuardsNeeded,
			PicName:          patrol.Location.PicName,
			PicPhone:         patrol.Location.PicPhone,
			Status:           patrol.Location.Status,
			CreatedAt:        patrol.Location.CreatedAt,
			UpdatedAt:        patrol.Location.UpdatedAt,
		}
		response.Location = &locationResponse
	}

	if patrol.Attendance != nil {
		attendanceResponse := model.AttendanceResponse{
			ID:                patrol.Attendance.ID,
			PersonnelID:       patrol.Attendance.PersonnelID,
			LocationID:        patrol.Attendance.LocationID,
			AssignmentID:      patrol.Attendance.AssignmentID,
			Date:              patrol.Attendance.Date.Format("2006-01-02"),
			CheckInLatitude:   patrol.Attendance.CheckInLatitude,
			CheckInLongitude:  patrol.Attendance.CheckInLongitude,
			CheckOutLatitude:  patrol.Attendance.CheckOutLatitude,
			CheckOutLongitude: patrol.Attendance.CheckOutLongitude,
			CheckInPhoto:      patrol.Attendance.CheckInPhoto,
			CheckOutPhoto:     patrol.Attendance.CheckOutPhoto,
			Status:            patrol.Attendance.Status,
			Notes:             patrol.Attendance.Notes,
			WorkDuration:      patrol.Attendance.WorkDuration,
			CreatedAt:         patrol.Attendance.CreatedAt,
			UpdatedAt:         patrol.Attendance.UpdatedAt,
		}
		if patrol.Attendance.CheckIn != nil {
			checkIn := patrol.Attendance.CheckIn.Format(time.RFC3339)
			attendanceResponse.CheckIn = &checkIn
		}
		if patrol.Attendance.CheckOut != nil {
			checkOut := patrol.Attendance.CheckOut.Format(time.RFC3339)
			attendanceResponse.CheckOut = &checkOut
		}
		response.Attendance = &attendanceResponse
	}

	if patrol.Validator != nil {
		validatorResponse := model.UserResponse{
			ID:        patrol.Validator.ID,
			BujpID:    patrol.Validator.BujpID,
			Email:     patrol.Validator.Email,
			Role:      patrol.Validator.Role,
			FullName:  patrol.Validator.FullName,
			Phone:     patrol.Validator.Phone,
			Photo:     patrol.Validator.Photo,
			Status:    patrol.Validator.Status,
			LastLogin: patrol.Validator.LastLogin,
			CreatedAt: patrol.Validator.CreatedAt,
			UpdatedAt: patrol.Validator.UpdatedAt,
		}
		response.Validator = &validatorResponse
	}

	return response
}
