package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type DashboardService interface {
	GetSummary(ctx context.Context, userID uuid.UUID, role string, bujpID uuid.UUID) (*model.DashboardSummary, error)
}

type dashboardService struct {
	userRepo       repository.UserRepository
	bujpRepo       repository.BujpRepository
	locationRepo   repository.LocationRepository
	personnelRepo  repository.PersonnelRepository
	assignmentRepo repository.AssignmentRepository
	attendanceRepo repository.AttendanceRepository
	leaveRepo      repository.LeaveRepository
}

func NewDashboardService(
	userRepo repository.UserRepository,
	bujpRepo repository.BujpRepository,
	locationRepo repository.LocationRepository,
	personnelRepo repository.PersonnelRepository,
	assignmentRepo repository.AssignmentRepository,
	attendanceRepo repository.AttendanceRepository,
	leaveRepo repository.LeaveRepository,
) DashboardService {
	return &dashboardService{
		userRepo:       userRepo,
		bujpRepo:       bujpRepo,
		locationRepo:   locationRepo,
		personnelRepo:  personnelRepo,
		assignmentRepo: assignmentRepo,
		attendanceRepo: attendanceRepo,
		leaveRepo:      leaveRepo,
	}
}

func (s *dashboardService) GetSummary(ctx context.Context, userID uuid.UUID, role string, bujpID uuid.UUID) (*model.DashboardSummary, error) {
	// Pusat (super_admin/admin) sees aggregates across all tenants; everyone
	// else is clamped to their own BUJP so dashboards do not leak data from
	// other BUJPs. The bujpID is part of the cache key so caches do not bleed
	// across tenants either.
	scopeBujpID := uuid.Nil
	if !utils.IsPusat(role) {
		scopeBujpID = bujpID
	}
	cacheKey := fmt.Sprintf("dashboard:summary:%s:%s:%s", role, userID.String(), scopeBujpID.String())
	var cached model.DashboardSummary
	if err := utils.CacheGet(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	} else if err != nil && !errors.Is(err, utils.ErrCacheMiss) {
		// fall through silently on cache errors
	}

	// BUJP-scoped roles without a BUJP linked must see an empty (but valid)
	// summary instead of leaking global counts.
	if utils.IsBujpScoped(role) && scopeBujpID == uuid.Nil {
		empty := &model.DashboardSummary{}
		_ = utils.CacheSet(ctx, cacheKey, empty, 60*time.Second)
		return empty, nil
	}

	summary := &model.DashboardSummary{}

	// Each sub-aggregation hits independent tables; run them in parallel
	// (with bounded concurrency) so the dashboard latency is dominated by the
	// single slowest query rather than the sum of all of them.
	var (
		overview         *model.Overview
		personnelStats   *model.PersonnelStats
		attendanceStats  *model.AttendanceStats
		leaveStats       *model.LeaveStats
		recentActivities []model.RecentActivity
	)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(6)

	g.Go(func() error {
		v, err := s.getOverview(gctx, scopeBujpID)
		if err != nil {
			return fmt.Errorf("failed to get overview: %w", err)
		}
		overview = v
		return nil
	})
	g.Go(func() error {
		v, err := s.getPersonnelStats(gctx, scopeBujpID)
		if err != nil {
			return fmt.Errorf("failed to get personnel stats: %w", err)
		}
		personnelStats = v
		return nil
	})
	g.Go(func() error {
		v, err := s.getAttendanceStats(gctx, scopeBujpID)
		if err != nil {
			return fmt.Errorf("failed to get attendance stats: %w", err)
		}
		attendanceStats = v
		return nil
	})
	g.Go(func() error {
		v, err := s.getLeaveStats(gctx, scopeBujpID)
		if err != nil {
			return fmt.Errorf("failed to get leave stats: %w", err)
		}
		leaveStats = v
		return nil
	})
	g.Go(func() error {
		v, err := s.getRecentActivities(gctx, 10, scopeBujpID)
		if err != nil {
			return fmt.Errorf("failed to get recent activities: %w", err)
		}
		recentActivities = v
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	summary.Overview = *overview
	summary.Personnel = *personnelStats
	summary.Attendance = *attendanceStats
	summary.Leaves = *leaveStats
	summary.RecentActivities = recentActivities

	// Notification feature disabled - set to zero stats
	summary.Notifications = model.NotificationStats{
		UnreadCount: 0,
		TotalCount:  0,
	}

	// Cache for 60s; tolerate write failures.
	_ = utils.CacheSet(ctx, cacheKey, summary, 60*time.Second)
	return summary, nil
}

// scopedFilter returns a filters map seeded with bujp_id when scopeBujpID is
// not nil. Callers must not mutate the returned map after passing it to repos
// in other goroutines.
func scopedFilter(scopeBujpID uuid.UUID, extras map[string]interface{}) map[string]interface{} {
	f := make(map[string]interface{}, len(extras)+1)
	if scopeBujpID != uuid.Nil {
		f["bujp_id"] = scopeBujpID
	}
	for k, v := range extras {
		f[k] = v
	}
	return f
}

func (s *dashboardService) getOverview(ctx context.Context, scopeBujpID uuid.UUID) (*model.Overview, error) {
	overview := &model.Overview{}

	// Count users
	_, total, err := s.userRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, nil))
	if err == nil {
		overview.TotalUsers = total
	}

	// Count BUJPs (only meaningful for Pusat; BUJP-scoped users see 1)
	if scopeBujpID == uuid.Nil {
		_, total, err = s.bujpRepo.FindAll(ctx, 1, 1, nil)
		if err == nil {
			overview.TotalBujps = total
		}
	} else {
		overview.TotalBujps = 1
	}

	// Count locations
	_, total, err = s.locationRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, nil))
	if err == nil {
		overview.TotalLocations = total
	}

	// Count personnel
	_, total, err = s.personnelRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, nil))
	if err == nil {
		overview.TotalPersonnel = total
	}

	// Count active guards (personnel with status active)
	_, total, err = s.personnelRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, map[string]interface{}{"status": "active"}))
	if err == nil {
		overview.ActiveGuards = total
	}

	// Count pending leaves
	_, total, err = s.leaveRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, map[string]interface{}{"status": "pending"}))
	if err == nil {
		overview.PendingLeaves = total
	}

	return overview, nil
}

func (s *dashboardService) getPersonnelStats(ctx context.Context, scopeBujpID uuid.UUID) (*model.PersonnelStats, error) {
	stats := &model.PersonnelStats{}

	// Total personnel
	_, total, err := s.personnelRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, nil))
	if err == nil {
		stats.TotalPersonnel = total
	}

	// Active personnel
	_, total, err = s.personnelRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, map[string]interface{}{"status": "active"}))
	if err == nil {
		stats.ActivePersonnel = total
	}

	// Inactive personnel
	stats.InactivePersonnel = stats.TotalPersonnel - stats.ActivePersonnel

	// Active assignments
	_, total, err = s.assignmentRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, map[string]interface{}{"status": "active"}))
	if err == nil {
		stats.ActiveAssignments = total
	}

	return stats, nil
}

func (s *dashboardService) getAttendanceStats(ctx context.Context, scopeBujpID uuid.UUID) (*model.AttendanceStats, error) {
	stats := &model.AttendanceStats{}

	// Today's date as time.Time (the attendance repo expects time.Time, not
	// a formatted string). Truncate to start of day.
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Count today's attendance
	_, total, err := s.attendanceRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, map[string]interface{}{"date": today}))
	if err == nil {
		stats.TodayAttendance = total
	}

	// Expected attendance = active assignments
	_, total, err = s.assignmentRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, map[string]interface{}{"status": "active"}))
	if err == nil {
		stats.TodayExpected = total
	}

	// Calculate attendance rate
	if stats.TodayExpected > 0 {
		stats.AttendanceRate = float64(stats.TodayAttendance) / float64(stats.TodayExpected) * 100
	}

	// Pending corrections (you'll need to implement this in your repo if not exists)
	// For now, set to 0
	stats.PendingCorrections = 0

	return stats, nil
}

func (s *dashboardService) getLeaveStats(ctx context.Context, scopeBujpID uuid.UUID) (*model.LeaveStats, error) {
	stats := &model.LeaveStats{}

	// Pending leaves
	_, total, err := s.leaveRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, map[string]interface{}{"status": "pending"}))
	if err == nil {
		stats.PendingLeaves = total
	}

	// Approved leaves
	_, total, err = s.leaveRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, map[string]interface{}{"status": "approved"}))
	if err == nil {
		stats.ApprovedLeaves = total
	}

	// Rejected leaves
	_, total, err = s.leaveRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, map[string]interface{}{"status": "rejected"}))
	if err == nil {
		stats.RejectedLeaves = total
	}

	// Total this month (all statuses)
	_, total, err = s.leaveRepo.FindAll(ctx, 1, 1, scopedFilter(scopeBujpID, nil))
	if err == nil {
		stats.TotalThisMonth = total
	}

	return stats, nil
}

// getNotificationStats is disabled - notification feature removed
// func (s *dashboardService) getNotificationStats(ctx context.Context, userID uuid.UUID) (*model.NotificationStats, error) {
// 	stats := &model.NotificationStats{}
// 	// Unread notifications
// 	unreadCount, err := s.notificationRepo.GetUnreadCount(ctx, userID)
// 	if err == nil {
// 		stats.UnreadCount = unreadCount
// 	}
// 	// Total notifications
// 	notifications, _, err := s.notificationRepo.GetAllByUser(ctx, userID, 1, 1, false)
// 	if err == nil {
// 		stats.TotalCount = int64(len(notifications))
// 	}
// 	return stats, nil
// }

func (s *dashboardService) getRecentActivities(ctx context.Context, limit int, scopeBujpID uuid.UUID) ([]model.RecentActivity, error) {
	activities := []model.RecentActivity{}

	// Get recent leaves
	leaves, _, err := s.leaveRepo.FindAll(ctx, 1, limit, scopedFilter(scopeBujpID, nil))
	if err == nil && leaves != nil {
		for _, leave := range leaves {
			activity := model.RecentActivity{
				Type:        "leave",
				Description: fmt.Sprintf("Leave request: %s", leave.Status),
				CreatedAt:   leave.CreatedAt.Format("2006-01-02 15:04:05"),
			}
			activities = append(activities, activity)
		}
	}

	// Get recent attendances
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	attendances, _, err := s.attendanceRepo.FindAll(ctx, 1, 5, scopedFilter(scopeBujpID, map[string]interface{}{"date": today}))
	if err == nil && attendances != nil {
		for _, att := range attendances {
			activity := model.RecentActivity{
				Type:        "attendance",
				Description: "Check-in recorded",
				CreatedAt:   att.CreatedAt.Format("2006-01-02 15:04:05"),
			}
			activities = append(activities, activity)
		}
	}

	// Limit to requested number
	if len(activities) > limit {
		activities = activities[:limit]
	}

	return activities, nil
}
