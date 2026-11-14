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
	Summary(ctx context.Context, filters map[string]interface{}) (*model.AttendanceSummary, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.AttendanceResponse, error)
	GetByPersonnelAndDate(ctx context.Context, personnelID uuid.UUID, date time.Time) (*model.AttendanceResponse, error)
	GetByPersonnelID(ctx context.Context, personnelID uuid.UUID, page, limit int) ([]model.AttendanceResponse, int64, error)
	GetByLocationID(ctx context.Context, locationID uuid.UUID, page, limit int) ([]model.AttendanceResponse, int64, error)
	GetByDateRange(ctx context.Context, startDate, endDate time.Time, page, limit int) ([]model.AttendanceResponse, int64, error)
	Create(ctx context.Context, req *model.CreateAttendanceRequest) (*model.AttendanceResponse, error)
	// ClockIn/ClockOut are idempotent: replayed is true when the answer is an earlier result, not a new write.
	ClockIn(ctx context.Context, personnelID uuid.UUID, req *model.CreateAttendanceRequest) (resp *model.AttendanceResponse, replayed bool, err error)
	ClockOut(ctx context.Context, personnelID uuid.UUID, req *model.UpdateAttendanceRequest) (resp *model.AttendanceResponse, replayed bool, err error)
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

// Summary tallies attendance for the filtered range in one query. "Hadir"
// counts everyone who showed up, late or not; "Izin" folds leave, permission
// and sick together, with Sakit also reported on its own.
func (s *attendanceServiceImpl) Summary(ctx context.Context, filters map[string]interface{}) (*model.AttendanceSummary, error) {
	c, err := s.repo.CountByStatus(ctx, filters)
	if err != nil {
		return nil, err
	}
	var total int64
	for _, n := range c {
		total += n
	}
	return &model.AttendanceSummary{
		Hadir:      c["present"] + c["late"],
		TepatWaktu: c["present"],
		Telat:      c["late"],
		Izin:       c["permission"] + c["sick"] + c["leave"],
		Sakit:      c["sick"],
		Alpa:       c["absent"],
		Total:      total,
	}, nil
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
		// Arriving after the shift started (plus the grace period) makes the day "late".
		if s.arrivedLate(ctx, req.AssignmentID, checkIn, date) {
			status = "late"
		}
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

// ClockIn records the signed-in employee's check-in. The server decides the day, the time and the
// status: whatever the client sends for date, check_in, check_out or status is ignored.
//
// It is idempotent. The row is written with INSERT ... ON CONFLICT DO NOTHING; if the employee
// already has a row for the day (a retry, a double tap, two requests at once) the existing row is
// answered as a success and replayed is true. Cost: the active assignment (office and shift in one
// query) and one INSERT.
func (s *attendanceServiceImpl) ClockIn(ctx context.Context, personnelID uuid.UUID, req *model.CreateAttendanceRequest) (*model.AttendanceResponse, bool, error) {
	if personnelID == uuid.Nil {
		return nil, false, errors.New("personnel record for current user not found")
	}
	now := time.Now()
	day := attendanceDay(now)

	assignment, err := s.assignmentRepo.FindActiveForAttendance(ctx, personnelID, day)
	if err != nil || assignment == nil {
		return nil, false, errors.New("no active assignment for current user")
	}
	if err := checkGeofence(assignment.Location, req.CheckInLatitude, req.CheckInLongitude); err != nil {
		return nil, false, err
	}

	status := "present"
	if assignment.Shift != nil && !assignment.Shift.StartTime.IsZero() &&
		isLateCheckIn(day, assignment.Shift.StartTime.Time, now, attendanceLateGraceMinutes) {
		status = "late"
	}
	key := req.IdempotencyKey
	if key == "" {
		key = deriveIdempotencyKey(personnelID, day, "checkin")
	}

	attendance := &model.Attendance{
		PersonnelID:        personnelID,
		LocationID:         assignment.LocationID,
		AssignmentID:       &assignment.ID,
		Date:               day,
		CheckIn:            &now,
		CheckInLatitude:    req.CheckInLatitude.PtrString(),
		CheckInLongitude:   req.CheckInLongitude.PtrString(),
		CheckInPhoto:       req.CheckInPhoto,
		SupportingDocument: req.SupportingDocument,
		Status:             status,
		Notes:              req.Notes,
		IdempotencyKey:     &key,
	}
	inserted, err := s.repo.InsertIdempotent(ctx, attendance)
	if err != nil {
		return nil, false, err
	}
	if !inserted {
		existing, err := s.repo.FindPlainByPersonnelAndDate(ctx, personnelID, day)
		if err != nil {
			return nil, false, err
		}
		attendance = existing
	}
	attendance.Location = assignment.Location
	attendance.Assignment = assignment

	response := toAttendanceResponse(attendance)
	presignAttendancePhotos(ctx, &response)
	return &response, !inserted, nil
}

// ClockOut records the check-out for the employee's open attendance. The time is the server's. The
// UPDATE only applies while check_out is still empty, so it is atomic: of any number of concurrent
// requests exactly one writes. A repeat that carries the same idempotency key (the header, or the
// per-day derived key when there is none) is answered with the stored result; a different key on an
// already-closed attendance is a conflict.
func (s *attendanceServiceImpl) ClockOut(ctx context.Context, personnelID uuid.UUID, req *model.UpdateAttendanceRequest) (*model.AttendanceResponse, bool, error) {
	if personnelID == uuid.Nil {
		return nil, false, errors.New("personnel record for current user not found")
	}
	now := time.Now()
	day := attendanceDay(now)

	rows, err := s.repo.FindOpenForCheckout(ctx, personnelID, day.AddDate(0, 0, -1), day)
	if err != nil {
		return nil, false, err
	}
	att := pickAttendanceToCheckOut(rows, day)
	if att == nil {
		return nil, false, ErrNoOpenAttendance
	}
	key := req.IdempotencyKey
	if key == "" {
		key = deriveIdempotencyKey(personnelID, att.Date, "checkout")
	}
	if att.CheckOut != nil {
		return s.replayCheckOut(ctx, att, key)
	}

	if att.Assignment != nil && att.Assignment.Shift != nil {
		start, end := att.Assignment.Shift.StartTime.Time, att.Assignment.Shift.EndTime.Time
		if !end.IsZero() && now.Before(shiftEnd(att.Date, start, end)) {
			return nil, false, fmt.Errorf("belum waktunya checkout, shift Anda berakhir pukul %s", end.Format("15:04"))
		}
	}
	if err := checkGeofence(att.Location, req.CheckOutLatitude, req.CheckOutLongitude); err != nil {
		return nil, false, err
	}

	duration := 0
	if att.CheckIn != nil {
		if d := int(now.Sub(*att.CheckIn).Minutes()); d > 0 {
			duration = d
		}
	}
	update := repository.CheckOutUpdate{
		At: now, Latitude: req.CheckOutLatitude.PtrString(), Longitude: req.CheckOutLongitude.PtrString(),
		Photo: req.CheckOutPhoto, Document: req.SupportingDocument, Notes: req.Notes, WorkDuration: duration, Key: key,
	}
	done, err := s.repo.CompleteCheckOut(ctx, att.ID, update)
	if err != nil {
		return nil, false, err
	}
	if !done {
		// Lost the race: someone closed it between our read and our write. Answer from what is stored.
		rows, err := s.repo.FindOpenForCheckout(ctx, personnelID, day.AddDate(0, 0, -1), day)
		if err != nil {
			return nil, false, err
		}
		if again := pickAttendanceToCheckOut(rows, day); again != nil && again.CheckOut != nil {
			return s.replayCheckOut(ctx, again, key)
		}
		return nil, false, ErrAlreadyCheckedOut
	}

	att.CheckOut = &now
	att.CheckOutLatitude, att.CheckOutLongitude = update.Latitude, update.Longitude
	if req.CheckOutPhoto != nil {
		att.CheckOutPhoto = req.CheckOutPhoto
	}
	att.WorkDuration = &duration
	response := toAttendanceResponse(att)
	presignAttendancePhotos(ctx, &response)
	return &response, false, nil
}

// replayCheckOut answers a clock-out for an attendance that is already closed.
func (s *attendanceServiceImpl) replayCheckOut(ctx context.Context, att *model.Attendance, key string) (*model.AttendanceResponse, bool, error) {
	if att.CheckOutIdempotencyKey == nil || *att.CheckOutIdempotencyKey != key {
		return nil, false, ErrAlreadyCheckedOut
	}
	response := toAttendanceResponse(att)
	presignAttendancePhotos(ctx, &response)
	return &response, true, nil
}

// pickAttendanceToCheckOut chooses today's row, or yesterday's only when it is still open and its
// shift runs overnight (so a 22:00-06:00 guard can check out after midnight).
func pickAttendanceToCheckOut(rows []model.Attendance, day time.Time) *model.Attendance {
	var yesterday *model.Attendance
	for i := range rows {
		switch {
		case rows[i].Date.Equal(day):
			return &rows[i]
		case rows[i].Date.Equal(day.AddDate(0, 0, -1)):
			yesterday = &rows[i]
		}
	}
	if yesterday != nil && yesterday.CheckOut == nil && yesterday.Assignment != nil && yesterday.Assignment.Shift != nil &&
		isOvernight(yesterday.Assignment.Shift.StartTime.Time, yesterday.Assignment.Shift.EndTime.Time) {
		return yesterday
	}
	return nil
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

// attendanceLateGraceMinutes is how long after the shift start a check-in still counts as on time.
const attendanceLateGraceMinutes = 5

// isLateCheckIn reports whether checkIn falls after shiftStart (a time of day) on the given date,
// allowing the grace period.
func isLateCheckIn(date time.Time, shiftStart time.Time, checkIn time.Time, graceMinutes int) bool {
	startAt := shiftClock(date, shiftStart)
	return checkIn.After(startAt.Add(time.Duration(graceMinutes) * time.Minute))
}

// arrivedLate looks up the assignment's shift. Without a check-in, an assignment or a shift there is
// nothing to compare against, so the day is not marked late.
func (s *attendanceServiceImpl) arrivedLate(ctx context.Context, assignmentID *uuid.UUID, checkIn *time.Time, date time.Time) bool {
	if checkIn == nil || assignmentID == nil || *assignmentID == uuid.Nil || s.assignmentRepo == nil {
		return false
	}
	assignment, err := s.assignmentRepo.FindByID(ctx, *assignmentID)
	if err != nil || assignment == nil || assignment.Shift == nil || assignment.Shift.StartTime.IsZero() {
		return false
	}
	return isLateCheckIn(date, assignment.Shift.StartTime.Time, *checkIn, attendanceLateGraceMinutes)
}
