package payroll

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	"github.com/ganiramadhan/ganipedia/backend/internal/constants"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	"github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/internal/services"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron       *cron.Cron
	payrollSvc services.PayrollService
	bujpRepo   repository.BujpRepository
}

func NewScheduler(payrollSvc services.PayrollService, bujpRepo repository.BujpRepository) *Scheduler {
	return &Scheduler{
		cron:       cron.New(cron.WithLocation(time.Local)),
		payrollSvc: payrollSvc,
		bujpRepo:   bujpRepo,
	}
}

func (s *Scheduler) Start() {
	if config.GetEnv("PAYROLL_CRON_ENABLED", "false") != "true" {
		log.Println("[payroll-cron] disabled (set PAYROLL_CRON_ENABLED=true to enable)")
		return
	}

	day := atoiDefault(config.GetEnv("PAYROLL_CRON_DAY", "25"), 25, 1, 28)
	hour := atoiDefault(config.GetEnv("PAYROLL_CRON_HOUR", "2"), 2, 0, 23)
	minute := atoiDefault(config.GetEnv("PAYROLL_CRON_MINUTE", "0"), 0, 0, 59)
	expr := fmt.Sprintf("%d %d %d * *", minute, hour, day)

	if _, err := s.cron.AddFunc(expr, s.run); err != nil {
		log.Printf("[payroll-cron] failed to register cron (%q): %v", expr, err)
		return
	}
	s.cron.Start()
	log.Printf("[payroll-cron] scheduled monthly auto-generate at \"%s\" (local time)", expr)
}

func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("[payroll-cron] stopped")
}

func (s *Scheduler) RunNow() { s.run() }

func (s *Scheduler) run() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	period := time.Now().Format(constants.PayrollPeriodFormat)
	log.Printf("[payroll-cron] starting auto-generate for period %s", period)

	bujps, _, err := s.bujpRepo.FindAll(ctx, 1, 1000, map[string]interface{}{"status": "active"})
	if err != nil {
		log.Printf("[payroll-cron] failed to list BUJPs: %v", err)
		return
	}

	for _, b := range bujps {
		req := &model.GeneratePayrollRequest{Period: period, BujpID: b.ID, Force: false}
		res, err := s.payrollSvc.Generate(ctx, req, nil)
		if err != nil {
			log.Printf("[payroll-cron] BUJP %s (%s) FAILED: %v", b.Name, b.ID, err)
			continue
		}
		log.Printf("[payroll-cron] BUJP %s — generated=%d skipped=%d", b.Name, len(res.Generated), len(res.Skipped))
	}
}

func atoiDefault(raw string, def, min, max int) int {
	v, err := strconv.Atoi(raw)
	if err != nil || v < min || v > max {
		return def
	}
	return v
}
