package model

// DashboardSummary represents the dashboard summary response
type DashboardSummary struct {
	Overview         Overview          `json:"overview"`
	Personnel        PersonnelStats    `json:"personnel"`
	Attendance       AttendanceStats   `json:"attendance"`
	Leaves           LeaveStats        `json:"leaves"`
	Notifications    NotificationStats `json:"notifications"`
	RecentActivities []RecentActivity  `json:"recent_activities"`
}

// Overview represents general system statistics
type Overview struct {
	TotalUsers     int64 `json:"total_users"`
	TotalBujps     int64 `json:"total_bujps"`
	TotalLocations int64 `json:"total_locations"`
	TotalPersonnel int64 `json:"total_personnel"`
	ActiveGuards   int64 `json:"active_guards"`
	PendingLeaves  int64 `json:"pending_leaves"`
}

// PersonnelStats represents personnel statistics
type PersonnelStats struct {
	TotalPersonnel    int64 `json:"total_personnel"`
	ActivePersonnel   int64 `json:"active_personnel"`
	InactivePersonnel int64 `json:"inactive_personnel"`
	ActiveAssignments int64 `json:"active_assignments"`
}

// AttendanceStats represents attendance statistics
type AttendanceStats struct {
	TodayAttendance    int64   `json:"today_attendance"`
	TodayExpected      int64   `json:"today_expected"`
	AttendanceRate     float64 `json:"attendance_rate"`
	PendingCorrections int64   `json:"pending_corrections"`
}

// LeaveStats represents leave statistics
type LeaveStats struct {
	PendingLeaves  int64 `json:"pending_leaves"`
	ApprovedLeaves int64 `json:"approved_leaves"`
	RejectedLeaves int64 `json:"rejected_leaves"`
	TotalThisMonth int64 `json:"total_this_month"`
}

// NotificationStats represents notification statistics
type NotificationStats struct {
	UnreadCount int64 `json:"unread_count"`
	TotalCount  int64 `json:"total_count"`
}

// RecentActivity represents a recent activity item
type RecentActivity struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UserName    string `json:"user_name,omitempty"`
}
