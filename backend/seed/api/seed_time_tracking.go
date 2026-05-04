package api

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// seedTimeTrackingHistoryStep populates Soll/Ist data for the demo tenant so
// the staff detail page (Dienstplan, KPI cards, history) has realistic numbers
// to render. It writes:
//   - one StaffWorkSchedule per Mo-Fr per staff (480 min target = 8 h)
//   - 14 calendar-day window of WorkSession + WorkSessionBreak rows per staff
//   - a single sick day for a quarter of the staff
//   - a 3-day vacation block for the first staff member
//
// The step skips gracefully when rt.DB is nil (remote seeding) or when no
// staff IDs were captured by the bootstrap step.
type seedTimeTrackingHistoryStep struct{}

func (seedTimeTrackingHistoryStep) Name() string { return "Seeding time-tracking history" }

const (
	timeTrackingDaysBack       = 14
	timeTrackingDailyTargetMin = 480
	timeTrackingDailyBreakMin  = 30
)

func (seedTimeTrackingHistoryStep) Run(ctx context.Context, rt *Runtime) error {
	if rt.DB == nil {
		fmt.Println("  skipping: no DB handle attached (remote seeder mode)")
		return nil
	}
	if rt.Bootstrap == nil || rt.Bootstrap.SchoolID == 0 {
		return errors.New("bootstrap school ID not available")
	}
	if rt.FixedSeeder == nil || len(rt.FixedSeeder.staffIDs) == 0 {
		fmt.Println("  skipping: no staff IDs available")
		return nil
	}

	tenantID := rt.Bootstrap.SchoolID
	tenantCtx := tenant.WithTenantID(ctx, tenantID)
	rng := rand.New(rand.NewPCG(uint64(tenantID), 0xC0FFEE))

	staffIDs := make([]int64, 0, len(rt.FixedSeeder.staffIDs))
	for _, id := range rt.FixedSeeder.staffIDs {
		staffIDs = append(staffIDs, id)
	}

	today := timezone.TodayUTC()
	location := timezone.Berlin

	scheduleCount := 0
	sessionCount := 0
	breakCount := 0
	absenceCount := 0

	for idx, staffID := range staffIDs {
		if err := seedScheduleForStaff(tenantCtx, rt.DB, tenantID, staffID, today); err != nil {
			return fmt.Errorf("seed schedule for staff %d: %w", staffID, err)
		}
		scheduleCount += 5

		// One vacation block for the first staff so the absence path has data.
		vacationDays := map[time.Time]struct{}{}
		if idx == 0 {
			start := mostRecentWeekday(today.AddDate(0, 0, -timeTrackingDaysBack+2), time.Tuesday)
			for offset := 0; offset < 3; offset++ {
				vacationDays[start.AddDate(0, 0, offset)] = struct{}{}
			}
			if err := insertAbsence(tenantCtx, rt.DB, tenantID, staffID,
				start, start.AddDate(0, 0, 2),
				activeModels.AbsenceTypeVacation, "Demo vacation block",
			); err != nil {
				return fmt.Errorf("seed vacation for staff %d: %w", staffID, err)
			}
			absenceCount++
		}

		var sickDay *time.Time
		if rng.Float64() < 0.25 {
			day := mostRecentWeekday(today.AddDate(0, 0, -rng.IntN(timeTrackingDaysBack)), time.Wednesday)
			if _, blocked := vacationDays[day]; !blocked {
				sickDay = &day
				if err := insertAbsence(tenantCtx, rt.DB, tenantID, staffID,
					day, day,
					activeModels.AbsenceTypeSick, "Demo sick day",
				); err != nil {
					return fmt.Errorf("seed sick day for staff %d: %w", staffID, err)
				}
				absenceCount++
			}
		}

		for offset := timeTrackingDaysBack - 1; offset >= 0; offset-- {
			day := today.AddDate(0, 0, -offset)
			if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
				continue
			}
			if _, vac := vacationDays[day]; vac {
				continue
			}
			if sickDay != nil && day.Equal(*sickDay) {
				continue
			}
			if rng.Float64() > 0.95 {
				continue
			}

			session, breaks, err := buildSession(rng, tenantID, staffID, day, location)
			if err != nil {
				return fmt.Errorf("build session for staff %d on %s: %w", staffID, day.Format("2006-01-02"), err)
			}
			if _, err := rt.DB.NewInsert().Model(session).ModelTableExpr("active.work_sessions").Exec(tenantCtx); err != nil {
				return fmt.Errorf("insert session for staff %d on %s: %w", staffID, day.Format("2006-01-02"), err)
			}
			sessionCount++
			for _, br := range breaks {
				br.SessionID = session.ID
				if _, err := rt.DB.NewInsert().Model(br).ModelTableExpr("active.work_session_breaks").Exec(tenantCtx); err != nil {
					return fmt.Errorf("insert break for session %d: %w", session.ID, err)
				}
				breakCount++
			}
		}
	}

	fmt.Printf("  %d schedules, %d sessions, %d breaks, %d absences seeded for %d staff\n",
		scheduleCount, sessionCount, breakCount, absenceCount, len(staffIDs))
	return nil
}

func seedScheduleForStaff(ctx context.Context, db *bun.DB, tenantID, staffID int64, validFrom time.Time) error {
	// Days since the most recent Monday: Mo=0, Tu=1, ..., Su=6.
	weekStart := validFrom.AddDate(0, 0, -((int(validFrom.Weekday()) + 6) % 7))
	for day := configModels.DayMonday; day <= configModels.DayFriday; day++ {
		schedule := &configModels.StaffWorkSchedule{
			TenantID:      tenantID,
			StaffID:       staffID,
			DayOfWeek:     day,
			TargetMinutes: timeTrackingDailyTargetMin,
			ValidFrom:     weekStart,
		}
		if _, err := db.NewInsert().
			Model(schedule).
			ModelTableExpr("config.staff_work_schedules").
			On("CONFLICT DO NOTHING").
			Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func buildSession(rng *rand.Rand, tenantID, staffID int64, day time.Time, loc *time.Location) (*activeModels.WorkSession, []*activeModels.WorkSessionBreak, error) {
	checkIn := time.Date(day.Year(), day.Month(), day.Day(), 8, rng.IntN(20)-10, 0, 0, loc)
	checkOutTarget := time.Date(day.Year(), day.Month(), day.Day(), 16, rng.IntN(30)-15, 0, 0, loc)

	status := activeModels.WorkSessionStatusPresent
	if rng.Float64() < 0.1 {
		status = activeModels.WorkSessionStatusHomeOffice
	}

	autoClosed := false
	var checkOut *time.Time
	if rng.Float64() < 0.05 {
		// Forgotten checkout: leave check_out_time NULL with auto_checked_out=true
		// so the UI can highlight the anomaly.
		autoClosed = true
	} else {
		c := checkOutTarget
		checkOut = &c
	}

	breakStart := time.Date(day.Year(), day.Month(), day.Day(), 12, rng.IntN(20)-10, 0, 0, loc)
	breakDuration := timeTrackingDailyBreakMin + rng.IntN(11) - 5 // 25..35 min
	breakEnd := breakStart.Add(time.Duration(breakDuration) * time.Minute)

	session := &activeModels.WorkSession{
		StaffID:        staffID,
		Date:           day,
		Status:         status,
		CheckInTime:    checkIn,
		CheckOutTime:   checkOut,
		BreakMinutes:   breakDuration,
		AutoCheckedOut: autoClosed,
		CreatedBy:      staffID,
	}
	session.SetTenantID(tenantID)

	br := &activeModels.WorkSessionBreak{
		StartedAt:       breakStart,
		EndedAt:         &breakEnd,
		DurationMinutes: breakDuration,
	}
	br.SetTenantID(tenantID)
	return session, []*activeModels.WorkSessionBreak{br}, nil
}

func insertAbsence(ctx context.Context, db *bun.DB, tenantID, staffID int64, dateStart, dateEnd time.Time, absenceType, note string) error {
	absence := &activeModels.StaffAbsence{
		StaffID:     staffID,
		AbsenceType: absenceType,
		DateStart:   dateStart,
		DateEnd:     dateEnd,
		Note:        note,
		Status:      activeModels.AbsenceStatusReported,
		CreatedBy:   staffID,
	}
	absence.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(absence).ModelTableExpr("active.staff_absences").Exec(ctx)
	return err
}

func mostRecentWeekday(from time.Time, target time.Weekday) time.Time {
	delta := int(from.Weekday() - target)
	if delta < 0 {
		delta += 7
	}
	return from.AddDate(0, 0, -delta)
}
