package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type attendanceCorrectionSQLLogger struct {
	logger.Interface
	queries []string
}

func (l *attendanceCorrectionSQLLogger) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	query, _ := fc()
	l.queries = append(l.queries, query)
}

func TestFindAllQualifiesColumnsForTenantScopedQueries(t *testing.T) {
	sqlLogger := &attendanceCorrectionSQLLogger{
		Interface: logger.Default.LogMode(logger.Silent),
	}
	db, err := gorm.Open(
		postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test"}),
		&gorm.Config{
			DryRun:               true,
			DisableAutomaticPing: true,
			Logger:               sqlLogger,
		},
	)
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}

	_, _, err = NewAttendanceCorrectionRepository(db).FindAll(
		context.Background(),
		1,
		10,
		map[string]interface{}{
			"bujp_id": uuid.New(),
			"status":  "pending",
		},
	)
	if err != nil {
		t.Fatalf("FindAll returned an error: %v", err)
	}

	queries := strings.Join(sqlLogger.queries, "\n")
	if !strings.Contains(queries, "attendance_corrections.status =") {
		t.Fatalf("status filter is not qualified in generated SQL: %s", queries)
	}
	if !strings.Contains(queries, "ORDER BY attendance_corrections.created_at DESC") {
		t.Fatalf("created_at order is not qualified in generated SQL: %s", queries)
	}
}
