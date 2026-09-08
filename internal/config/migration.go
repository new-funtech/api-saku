package config

import (
	"log"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
)

var tablesWithSoftDelete = []string{
	"users",
	"bujps",
	"locations",
	"shifts",
	"personnels",
	"products",
	"salary_components",
	"payrolls",
	"payroll_details",
	"monthly_reports",
	"loan_products",
	"loans",
	"loan_approvals",
	"loan_installments",
}

func RunMigrations() {
	if DB == nil {
		log.Println("Warning: Database not connected, skipping migrations")
		return
	}

	dropSoftDeleteColumns()

	// Auto migrate all models
	if err := DB.AutoMigrate(
		&model.Product{},
		&model.Bujp{},
		&model.User{},
		&model.Location{},
		&model.Shift{},
		&model.Personnel{},
		&model.Assignment{},
		&model.Attendance{},
		&model.AttendanceCorrection{},
		&model.Patrol{},
		&model.Leave{},
		&model.SalaryComponent{},
		&model.Payroll{},
		&model.PayrollDetail{},
		&model.MonthlyReport{},
		&model.LoanProduct{},
		&model.Loan{},
		&model.LoanApproval{},
		&model.LoanInstallment{},
		&model.Notification{}); err != nil {
		log.Printf("AutoMigrate warning: %v", err)
	}

	if err := fixTimeColumns(); err != nil {
		log.Printf("Warning: fix time columns: %v", err)
	}

	dropOldIndexes := []string{
		`DROP INDEX IF EXISTS idx_users_email CASCADE`,
		`DROP INDEX IF EXISTS idx_bujps_code CASCADE`,
		`DROP INDEX IF EXISTS idx_locations_code CASCADE`,
		`DROP INDEX IF EXISTS idx_personnels_id_number CASCADE`,
		`DROP INDEX IF EXISTS idx_salary_components_code CASCADE`,
		`DROP INDEX IF EXISTS idx_payrolls_personnel_period CASCADE`,
		`DROP INDEX IF EXISTS idx_monthly_reports_bujp_period CASCADE`,
		`DROP INDEX IF EXISTS idx_loan_products_code CASCADE`,
		`DROP INDEX IF EXISTS idx_loans_loan_number CASCADE`,
	}
	for _, sql := range dropOldIndexes {
		if err := DB.Exec(sql).Error; err != nil {
			log.Printf("Warning: dropping old index: %v", err)
		}
	}

	indexes := []string{
		// Users
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_bujp_id ON users(bujp_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_role ON users(role)`,

		// Bujps
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_bujps_code ON bujps(code)`,
		`CREATE INDEX IF NOT EXISTS idx_bujps_status ON bujps(status)`,

		// Locations
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_locations_code ON locations(code)`,
		`CREATE INDEX IF NOT EXISTS idx_locations_bujp_id ON locations(bujp_id)`,
		`CREATE INDEX IF NOT EXISTS idx_locations_status ON locations(status)`,

		// Shifts
		`CREATE INDEX IF NOT EXISTS idx_shifts_bujp_id ON shifts(bujp_id)`,
		`CREATE INDEX IF NOT EXISTS idx_shifts_status ON shifts(status)`,

		// Personnels
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_personnels_id_number ON personnels(id_number)`,
		`CREATE INDEX IF NOT EXISTS idx_personnels_bujp_id ON personnels(bujp_id)`,
		`CREATE INDEX IF NOT EXISTS idx_personnels_user_id ON personnels(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_personnels_status ON personnels(status)`,

		// Assignments
		`CREATE INDEX IF NOT EXISTS idx_assignments_personnel_id ON assignments(personnel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_assignments_location_id ON assignments(location_id)`,
		`CREATE INDEX IF NOT EXISTS idx_assignments_shift_id ON assignments(shift_id)`,
		`CREATE INDEX IF NOT EXISTS idx_assignments_status ON assignments(status)`,
		`CREATE INDEX IF NOT EXISTS idx_assignments_start_date ON assignments(start_date)`,

		// Attendances
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_attendances_personnel_date ON attendances(personnel_id, date)`,
		`CREATE INDEX IF NOT EXISTS idx_attendances_location_id ON attendances(location_id)`,
		`CREATE INDEX IF NOT EXISTS idx_attendances_assignment_id ON attendances(assignment_id)`,
		`CREATE INDEX IF NOT EXISTS idx_attendances_date ON attendances(date)`,
		`CREATE INDEX IF NOT EXISTS idx_attendances_status ON attendances(status)`,

		// Attendance Corrections
		`CREATE INDEX IF NOT EXISTS idx_attendance_corrections_personnel_id ON attendance_corrections(personnel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_attendance_corrections_status ON attendance_corrections(status)`,
		`CREATE INDEX IF NOT EXISTS idx_attendance_corrections_correction_date ON attendance_corrections(correction_date)`,

		// Patrols
		`CREATE INDEX IF NOT EXISTS idx_patrols_personnel_id ON patrols(personnel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_patrols_location_id ON patrols(location_id)`,
		`CREATE INDEX IF NOT EXISTS idx_patrols_attendance_id ON patrols(attendance_id)`,
		`CREATE INDEX IF NOT EXISTS idx_patrols_date ON patrols(date)`,
		`CREATE INDEX IF NOT EXISTS idx_patrols_validation_status ON patrols(validation_status)`,

		// Leaves
		`CREATE INDEX IF NOT EXISTS idx_leaves_personnel_id ON leaves(personnel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_leaves_status ON leaves(status)`,
		`CREATE INDEX IF NOT EXISTS idx_leaves_type ON leaves(type)`,
		`CREATE INDEX IF NOT EXISTS idx_leaves_start_date ON leaves(start_date)`,
		`CREATE INDEX IF NOT EXISTS idx_leaves_end_date ON leaves(end_date)`,

		// Salary Components
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_salary_components_code ON salary_components(code)`,
		`CREATE INDEX IF NOT EXISTS idx_salary_components_bujp_id ON salary_components(bujp_id)`,
		`CREATE INDEX IF NOT EXISTS idx_salary_components_type ON salary_components(type)`,
		`CREATE INDEX IF NOT EXISTS idx_salary_components_is_active ON salary_components(is_active)`,

		// Payrolls
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_payrolls_personnel_period ON payrolls(personnel_id, period)`,
		`CREATE INDEX IF NOT EXISTS idx_payrolls_bujp_id ON payrolls(bujp_id)`,
		`CREATE INDEX IF NOT EXISTS idx_payrolls_period ON payrolls(period)`,
		`CREATE INDEX IF NOT EXISTS idx_payrolls_payment_status ON payrolls(payment_status)`,
		`CREATE INDEX IF NOT EXISTS idx_payrolls_created_by ON payrolls(created_by)`,

		// Payroll Details
		`CREATE INDEX IF NOT EXISTS idx_payroll_details_payroll_id ON payroll_details(payroll_id)`,
		`CREATE INDEX IF NOT EXISTS idx_payroll_details_salary_component_id ON payroll_details(salary_component_id)`,

		// Monthly Reports
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_monthly_reports_bujp_period ON monthly_reports(bujp_id, period)`,
		`CREATE INDEX IF NOT EXISTS idx_monthly_reports_bujp_id ON monthly_reports(bujp_id)`,
		`CREATE INDEX IF NOT EXISTS idx_monthly_reports_period ON monthly_reports(period)`,
		`CREATE INDEX IF NOT EXISTS idx_monthly_reports_created_by ON monthly_reports(created_by)`,

		// Loan Products
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_loan_products_code ON loan_products(code)`,
		`CREATE INDEX IF NOT EXISTS idx_loan_products_bujp_id ON loan_products(bujp_id)`,
		`CREATE INDEX IF NOT EXISTS idx_loan_products_is_active ON loan_products(is_active)`,

		// Loans
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_loans_loan_number ON loans(loan_number)`,
		`CREATE INDEX IF NOT EXISTS idx_loans_personnel_id ON loans(personnel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_loans_bujp_id ON loans(bujp_id)`,
		`CREATE INDEX IF NOT EXISTS idx_loans_loan_product_id ON loans(loan_product_id)`,
		`CREATE INDEX IF NOT EXISTS idx_loans_status ON loans(status)`,

		// Loan Approvals
		`CREATE INDEX IF NOT EXISTS idx_loan_approvals_loan_id ON loan_approvals(loan_id)`,
		`CREATE INDEX IF NOT EXISTS idx_loan_approvals_status ON loan_approvals(status)`,
		`CREATE INDEX IF NOT EXISTS idx_loan_approvals_approver_id ON loan_approvals(approver_id)`,

		// Loan Installments
		`CREATE INDEX IF NOT EXISTS idx_loan_installments_loan_id ON loan_installments(loan_id)`,
		`CREATE INDEX IF NOT EXISTS idx_loan_installments_status ON loan_installments(status)`,
		`CREATE INDEX IF NOT EXISTS idx_loan_installments_due_date ON loan_installments(due_date)`,
		`CREATE INDEX IF NOT EXISTS idx_loan_installments_payroll_id ON loan_installments(payroll_id)`,

		// Notifications
		`CREATE INDEX IF NOT EXISTS idx_notifications_recipient_user_id ON notifications(recipient_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_is_read ON notifications(is_read)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_entity_id ON notifications(entity_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_event_recipient_type ON notifications(event_id, recipient_user_id, type)`,
	}

	for _, idx := range indexes {
		if err := DB.Exec(idx).Error; err != nil {
			log.Printf("Warning: index creation: %v", err)
		}
	}

	log.Println("Database migration completed successfully")
}

func dropSoftDeleteColumns() {
	for _, table := range tablesWithSoftDelete {
		if err := DB.Exec(`ALTER TABLE IF EXISTS ` + table + ` DROP COLUMN IF EXISTS deleted_at CASCADE`).Error; err != nil {
			log.Printf("Warning: dropping deleted_at on %s: %v", table, err)
		}
	}
}

func fixTimeColumns() error {
	var dataType string
	err := DB.Raw(`
		SELECT data_type 
		FROM information_schema.columns 
		WHERE table_name = 'shifts' 
		AND column_name = 'start_time'
	`).Scan(&dataType).Error

	if err != nil {
		return err
	}

	if dataType == "timestamp with time zone" || dataType == "timestamp without time zone" {
		log.Println("Fixing shift time columns from timestamp to time type...")

		migrations := []string{
			`ALTER TABLE shifts DROP COLUMN IF EXISTS start_time CASCADE`,
			`ALTER TABLE shifts DROP COLUMN IF EXISTS end_time CASCADE`,
			`ALTER TABLE shifts ADD COLUMN start_time TIME NOT NULL DEFAULT '00:00:00'`,
			`ALTER TABLE shifts ADD COLUMN end_time TIME NOT NULL DEFAULT '00:00:00'`,
			`ALTER TABLE shifts ALTER COLUMN start_time DROP DEFAULT`,
			`ALTER TABLE shifts ALTER COLUMN end_time DROP DEFAULT`,
		}

		for _, sql := range migrations {
			if err := DB.Exec(sql).Error; err != nil {
				return err
			}
		}

		log.Println("Time columns fixed successfully")
	}

	return nil
}
