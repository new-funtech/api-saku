package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type StatsHandler struct {
	db *gorm.DB
}

func NewStatsHandler(db *gorm.DB) *StatsHandler { return &StatsHandler{db: db} }

func statsCompany(c *fiber.Ctx, allowAll bool) (company uuid.UUID, all bool, denied error, handled bool) {
	if !utils.IsPusat(utils.CallerRole(c)) {
		own := utils.CallerBujpID(c)
		if own == uuid.Nil {
			return uuid.Nil, false, forbiddenResponse(c), true
		}
		return own, false, nil, false
	}
	if v := strings.TrimSpace(c.Query("bujp_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return uuid.Nil, false, c.Status(http.StatusBadRequest).JSON(model.APIResponse{Status: "error", Code: http.StatusBadRequest, Message: "bujp_id is invalid"}), true
		}
		return id, false, nil, false
	}
	if allowAll {
		return uuid.Nil, true, nil, false
	}
	return uuid.Nil, false, c.Status(http.StatusBadRequest).JSON(model.APIResponse{Status: "error", Code: http.StatusBadRequest, Message: "bujp_id wajib diisi"}), true
}

func parseDay(v, fallback string) (time.Time, error) {
	if strings.TrimSpace(v) == "" {
		v = fallback
	}
	return time.Parse("2006-01-02", v)
}

type attendanceStats struct {
	From       string          `json:"from"`
	To         string          `json:"to"`
	People     int64           `json:"people"`
	Total      int64           `json:"total"`
	Present    int64           `json:"present"` // present + late
	Late       int64           `json:"late"`
	Leave      int64           `json:"leave"` // permission + sick + leave
	Absent     int64           `json:"absent"`
	ByWeekday  []weekdayCount  `json:"by_weekday"`
	ByLocation []locationCount `json:"by_location"`
	ByDay      []dayCount      `json:"by_day"`
}

type weekdayCount struct {
	Dow     int   `json:"dow"` // 0 = Sunday
	Present int64 `json:"present"`
}

type locationCount struct {
	LocationID uuid.UUID `json:"location_id"`
	Name       string    `json:"name"`
	Present    int64     `json:"present"`
	Total      int64     `json:"total"`
}

type dayCount struct {
	Date    string `json:"date"`
	Present int64  `json:"present"`
	Late    int64  `json:"late"`
	Leave   int64  `json:"leave"`
	Absent  int64  `json:"absent"`
}

// Attendance returns attendance counts for a date range (default: this month).
//
// @Router /attendances/stats [get]
func (h *StatsHandler) Attendance(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	company, _, err, handled := statsCompany(c, false)
	if handled {
		return err
	}
	now := time.Now()
	from, ferr := parseDay(c.Query("date_from"), now.Format("2006-01")+"-01")
	to, terr := parseDay(c.Query("date_to"), now.Format("2006-01-02"))
	if ferr != nil || terr != nil || to.Before(from) || to.Sub(from) > 366*24*time.Hour {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{Status: "error", Code: http.StatusBadRequest, Message: "date_from/date_to tidak valid (YYYY-MM-DD, maksimal 1 tahun)"})
	}
	var location uuid.UUID
	if v := c.Query("location_id"); v != "" {
		id, perr := uuid.Parse(v)
		if perr != nil || !locationAllowed(c, ctx, id) {
			return forbiddenResponse(c)
		}
		location = id
	}

	scope := "p.bujp_id = ? AND a.date BETWEEN ? AND ?"
	args := []interface{}{company, from, to}
	if location != uuid.Nil {
		scope += " AND a.location_id = ?"
		args = append(args, location)
	}
	from2 := " FROM attendances a JOIN personnels p ON p.id = a.personnel_id "
	out := attendanceStats{From: from.Format("2006-01-02"), To: to.Format("2006-01-02"), ByWeekday: []weekdayCount{}, ByLocation: []locationCount{}, ByDay: []dayCount{}}

	var tot struct{ Total, Present, Late, Leave, Absent int64 }
	if e := h.db.WithContext(ctx).Raw(`SELECT count(*) AS total,
		count(*) FILTER (WHERE a.status IN ('present','late')) AS present,
		count(*) FILTER (WHERE a.status = 'late') AS late,
		count(*) FILTER (WHERE a.status IN ('permission','sick','leave')) AS leave,
		count(*) FILTER (WHERE a.status = 'absent') AS absent`+from2+"WHERE "+scope, args...).Scan(&tot).Error; e != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{Status: "error", Code: http.StatusInternalServerError, Message: "Gagal memuat statistik"})
	}
	out.Total, out.Present, out.Late, out.Leave, out.Absent = tot.Total, tot.Present, tot.Late, tot.Leave, tot.Absent

	_ = h.db.WithContext(ctx).Raw(`SELECT count(*) FROM personnels WHERE bujp_id = ? AND status IN ('active','on_leave')`, company).Scan(&out.People).Error
	_ = h.db.WithContext(ctx).Raw(`SELECT EXTRACT(dow FROM a.date)::int AS dow, count(*) FILTER (WHERE a.check_in IS NOT NULL) AS present`+from2+"WHERE "+scope+" GROUP BY 1 ORDER BY 1", args...).Scan(&out.ByWeekday).Error
	_ = h.db.WithContext(ctx).Raw(`SELECT l.id AS location_id, l.name,
		count(*) FILTER (WHERE a.status IN ('present','late')) AS present, count(*) AS total`+from2+"JOIN locations l ON l.id = a.location_id WHERE "+scope+" GROUP BY l.id, l.name ORDER BY total DESC LIMIT 20", args...).Scan(&out.ByLocation).Error
	_ = h.db.WithContext(ctx).Raw(`SELECT to_char(a.date, 'YYYY-MM-DD') AS date,
		count(*) FILTER (WHERE a.status IN ('present','late')) AS present,
		count(*) FILTER (WHERE a.status = 'late') AS late,
		count(*) FILTER (WHERE a.status IN ('permission','sick','leave')) AS leave,
		count(*) FILTER (WHERE a.status = 'absent') AS absent`+from2+"WHERE "+scope+" GROUP BY a.date ORDER BY a.date", args...).Scan(&out.ByDay).Error

	return c.Status(http.StatusOK).JSON(model.APIResponse{Status: "success", Code: http.StatusOK, Message: "Statistik kehadiran", Data: out})
}

type loanStats struct {
	Since    string          `json:"since"`
	Funds    float64         `json:"funds"`    // paid out (disbursed, active, completed)
	Interest float64         `json:"interest"` // total repayment minus principal, same loans
	Total    int64           `json:"total"`
	Running  int64           `json:"running"` // disbursed + active
	Waiting  int64           `json:"waiting"`
	ByStatus []statusCount   `json:"by_status"`
	ByMonth  []monthlyAmount `json:"by_month"`
}

type statusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type monthlyAmount struct {
	Month string  `json:"month"` // YYYY-MM
	Funds float64 `json:"funds"`
}

func (h *StatsHandler) Loans(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	company, all, err, handled := statsCompany(c, true)
	if handled {
		return err
	}
	def := time.Now().AddDate(0, -2, 0)
	since, serr := parseDay(c.Query("since"), time.Date(def.Year(), def.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"))
	if serr != nil {
		return c.Status(http.StatusBadRequest).JSON(model.APIResponse{Status: "error", Code: http.StatusBadRequest, Message: "since tidak valid (YYYY-MM-DD)"})
	}
	where, args := "created_at >= ?", []interface{}{since}
	if !all {
		where += " AND bujp_id = ?"
		args = append(args, company)
	}
	const amount = "COALESCE(NULLIF(approved_amount, 0), loan_amount)"
	out := loanStats{Since: since.Format("2006-01-02"), ByStatus: []statusCount{}, ByMonth: []monthlyAmount{}}

	var tot struct {
		Funds, Interest float64
		Total, Running  int64
		Waiting         int64
	}
	if e := h.db.WithContext(ctx).Raw(`SELECT
		COALESCE(SUM(`+amount+`) FILTER (WHERE status IN ('disbursed','active','completed')), 0) AS funds,
		COALESCE(SUM(total_repayment - `+amount+`) FILTER (WHERE status IN ('disbursed','active','completed')), 0) AS interest,
		count(*) AS total,
		count(*) FILTER (WHERE status IN ('disbursed','active')) AS running,
		count(*) FILTER (WHERE status IN ('submitted','approved_bujp','approved_pusat')) AS waiting
		FROM loans WHERE `+where, args...).Scan(&tot).Error; e != nil {
		return c.Status(http.StatusInternalServerError).JSON(model.APIResponse{Status: "error", Code: http.StatusInternalServerError, Message: "Gagal memuat statistik"})
	}
	out.Funds, out.Interest, out.Total, out.Running, out.Waiting = tot.Funds, tot.Interest, tot.Total, tot.Running, tot.Waiting
	_ = h.db.WithContext(ctx).Raw(`SELECT status, count(*) AS count FROM loans WHERE `+where+` GROUP BY status ORDER BY status`, args...).Scan(&out.ByStatus).Error
	_ = h.db.WithContext(ctx).Raw(`SELECT to_char(created_at, 'YYYY-MM') AS month, COALESCE(SUM(`+amount+`), 0) AS funds
		FROM loans WHERE `+where+` AND status IN ('disbursed','active','completed') GROUP BY 1 ORDER BY 1`, args...).Scan(&out.ByMonth).Error

	return c.Status(http.StatusOK).JSON(model.APIResponse{Status: "success", Code: http.StatusOK, Message: "Statistik kredit", Data: out})
}
