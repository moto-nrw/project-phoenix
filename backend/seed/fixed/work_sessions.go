package fixed

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
)

// seedWorkSessions creates realistic historical work session data for the WeekChart.
// Generates sessions for the previous week through yesterday (skipping weekends and today).
// Each session gets 1-2 break records (morning + lunch) with break_minutes as cached total.
func (s *Seeder) seedWorkSessions(ctx context.Context) error {
	today := time.Now().Truncate(24 * time.Hour)
	monday := s.getPreviousWeekMonday(today)

	staffCount := min(5, len(s.result.Staff))
	if staffCount == 0 {
		if s.verbose {
			log.Printf("No staff found, skipping work sessions")
		}
		return nil
	}

	for staffIdx, staff := range s.result.Staff[:staffCount] {
		if err := s.seedStaffWorkSessions(ctx, staff.ID, staffIdx, monday, today); err != nil {
			return err
		}
	}

	if s.verbose {
		log.Printf("Created %d work sessions with %d breaks",
			len(s.result.WorkSessions), len(s.result.WorkSessionBreaks))
	}
	return nil
}

// getPreviousWeekMonday returns the Monday of the previous week for a given date.
func (s *Seeder) getPreviousWeekMonday(today time.Time) time.Time {
	weekday := today.Weekday()
	mondayOffset := int(weekday - time.Monday)
	if weekday == time.Sunday {
		mondayOffset = 6
	}
	return today.AddDate(0, 0, -mondayOffset-7)
}

// seedStaffWorkSessions creates work sessions for a single staff member.
func (s *Seeder) seedStaffWorkSessions(ctx context.Context, staffID int64, staffIdx int, monday, today time.Time) error {
	for d := monday; d.Before(today); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		if err := s.seedSingleWorkSession(ctx, staffID, staffIdx, d); err != nil {
			return err
		}
	}
	return nil
}

// seedSingleWorkSession creates a single work session with breaks for a given day.
func (s *Seeder) seedSingleWorkSession(ctx context.Context, staffID int64, staffIdx int, d time.Time) error {
	dayIdx := int(d.Weekday()) - 1 // Mon=0, Tue=1, ...

	checkIn := d.Add(7*time.Hour + 30*time.Minute + time.Duration(staffIdx*15)*time.Minute)
	checkOut := d.Add(15*time.Hour + 30*time.Minute + time.Duration(staffIdx*20+dayIdx*10)*time.Minute)

	status := active.WorkSessionStatusPresent
	if staffIdx%3 == 1 && dayIdx%2 == 0 {
		status = active.WorkSessionStatusHomeOffice
	}

	breaks := buildBreaksForSession(checkIn, checkOut, staffIdx, dayIdx)
	breakMins := sumBreakMinutes(breaks)

	session := &active.WorkSession{
		StaffID:      staffID,
		Date:         d,
		Status:       status,
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
		BreakMinutes: breakMins,
		CreatedBy:    staffID,
	}
	session.CreatedAt = time.Now()
	session.UpdatedAt = time.Now()

	if err := s.insertWorkSession(ctx, session); err != nil {
		return fmt.Errorf("failed to upsert work session for staff %d on %s: %w",
			staffID, d.Format("2006-01-02"), err)
	}

	s.result.WorkSessions = append(s.result.WorkSessions, session)

	return s.insertSessionBreaks(ctx, session.ID, breaks)
}

// insertWorkSession inserts or updates a work session record.
func (s *Seeder) insertWorkSession(ctx context.Context, session *active.WorkSession) error {
	_, err := s.tx.NewInsert().Model(session).
		ModelTableExpr("active.work_sessions").
		On("CONFLICT (staff_id, date) DO UPDATE").
		Set("check_in_time = EXCLUDED.check_in_time").
		Set("check_out_time = EXCLUDED.check_out_time").
		Set("break_minutes = EXCLUDED.break_minutes").
		Set("status = EXCLUDED.status").
		Set(SQLExcludedUpdatedAt).
		Returning(SQLBaseColumns).
		Exec(ctx)
	return err
}

// insertSessionBreaks inserts break records for a work session.
func (s *Seeder) insertSessionBreaks(ctx context.Context, sessionID int64, breaks []*active.WorkSessionBreak) error {
	for _, b := range breaks {
		b.SessionID = sessionID
		b.CreatedAt = time.Now()
		b.UpdatedAt = time.Now()

		_, err := s.tx.NewInsert().Model(b).
			ModelTableExpr("active.work_session_breaks").
			On("CONFLICT DO NOTHING").
			Returning(SQLBaseColumns).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to insert break for session %d: %w", sessionID, err)
		}

		s.result.WorkSessionBreaks = append(s.result.WorkSessionBreaks, b)
	}
	return nil
}

// sumBreakMinutes calculates total break minutes from break records.
func sumBreakMinutes(breaks []*active.WorkSessionBreak) int {
	total := 0
	for _, b := range breaks {
		total += b.DurationMinutes
	}
	return total
}

// buildBreaksForSession generates 1-2 realistic break records for a work session.
// Morning break: ~10:00 for 15 min (not every day)
// Lunch break: ~12:30 for 30 or 45 min (§4 ArbZG: ≥30min for >6h, ≥45min for >9h)
// All durations use multiples of 15 to match the frontend dropdown (0, 15, 30, 45, 60).
func buildBreaksForSession(checkIn, checkOut time.Time, staffIdx, dayIdx int) []*active.WorkSessionBreak {
	var breaks []*active.WorkSessionBreak
	netHours := checkOut.Sub(checkIn).Hours()
	day := checkIn.Truncate(24 * time.Hour)

	// Morning break: ~10:00, 15 min, only some staff/days
	if staffIdx%2 == 0 || dayIdx%3 == 0 {
		morningStart := day.Add(10*time.Hour + time.Duration(staffIdx*5)*time.Minute)
		morningDur := 15
		morningEnd := morningStart.Add(time.Duration(morningDur) * time.Minute)

		breaks = append(breaks, &active.WorkSessionBreak{
			StartedAt:       morningStart,
			EndedAt:         &morningEnd,
			DurationMinutes: morningDur,
		})
	}

	// Lunch break: ~12:30, always present, duration depends on total work time
	lunchStart := day.Add(12*time.Hour + 30*time.Minute + time.Duration(staffIdx*5)*time.Minute)
	lunchDur := 30
	if netHours > 9.5 {
		// Need ≥45min total break for >9h (§4 ArbZG)
		// Account for morning break if present
		morningTotal := 0
		for _, b := range breaks {
			morningTotal += b.DurationMinutes
		}
		lunchDur = int(math.Max(float64(45-morningTotal), 30))
	}

	lunchEnd := lunchStart.Add(time.Duration(lunchDur) * time.Minute)

	breaks = append(breaks, &active.WorkSessionBreak{
		StartedAt:       lunchStart,
		EndedAt:         &lunchEnd,
		DurationMinutes: lunchDur,
	})

	return breaks
}
