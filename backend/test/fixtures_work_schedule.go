package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// StaffWorkScheduleFixture is the part of a contractual schedule row that
// tests script: one weekday of a non-rotating schedule valid from a fixed
// date. Its ID is the handle for a later retroactive change.
type StaffWorkScheduleFixture struct {
	ID            int64
	StaffID       int64
	DayOfWeek     int
	TargetMinutes int
}

// CreateTestStaffWorkScheduleForTenant gives a staff member the contractual
// target minutes of one weekday (ISO: 0=Monday … 6=Sunday), valid from
// validFrom onwards with no rotation. Callers that need a whole week create
// one row per weekday.
func CreateTestStaffWorkScheduleForTenant(tb testing.TB, db *bun.DB, tenantID, staffID int64, dayOfWeek, targetMinutes int, validFrom timezone.Date) *StaffWorkScheduleFixture {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int64
	err := db.NewRaw(
		`INSERT INTO config.staff_work_schedules
		        (tenant_id, staff_id, day_of_week, target_minutes, week_index, rotation_length, valid_from)
		 VALUES (?, ?, ?, ?, 0, 1, ?)
		 RETURNING id`,
		tenantID, staffID, dayOfWeek, targetMinutes, validFrom.String(),
	).Scan(ctx, &id)
	require.NoError(tb, err, "Failed to create test staff work schedule")

	return &StaffWorkScheduleFixture{ID: id, StaffID: staffID, DayOfWeek: dayOfWeek, TargetMinutes: targetMinutes}
}

// SetStaffWorkScheduleTargetMinutes changes one schedule row's contractual
// target after the fact, which is how tests provoke Soll drift against an
// already closed month.
func SetStaffWorkScheduleTargetMinutes(tb testing.TB, db *bun.DB, ctx context.Context, scheduleID int64, targetMinutes int) {
	tb.Helper()
	_, err := db.NewRaw(
		`UPDATE config.staff_work_schedules SET target_minutes = ? WHERE id = ?`,
		targetMinutes, scheduleID,
	).Exec(ctx)
	require.NoError(tb, err, "Failed to change test staff work schedule target")
}
