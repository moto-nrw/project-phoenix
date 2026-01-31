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

	// Find Monday of PREVIOUS week to get ~2 weeks of data
	weekday := today.Weekday()
	mondayOffset := int(weekday - time.Monday)
	if weekday == time.Sunday {
		mondayOffset = 6
	}
	monday := today.AddDate(0, 0, -mondayOffset-7)

	staffCount := min(5, len(s.result.Staff))
	if staffCount == 0 {
		if s.verbose {
			log.Printf("No staff found, skipping work sessions")
		}
		return nil
	}

	for staffIdx, staff := range s.result.Staff[:staffCount] {
		for d := monday; d.Before(today); d = d.AddDate(0, 0, 1) {
			if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
				continue
			}

			dayIdx := int(d.Weekday()) - 1 // Mon=0, Tue=1, ...

			checkIn := d.Add(7*time.Hour + 30*time.Minute +
				time.Duration(staffIdx*15)*time.Minute)
			checkOut := d.Add(15*time.Hour + 30*time.Minute +
				time.Duration(staffIdx*20+dayIdx*10)*time.Minute)

			status := active.WorkSessionStatusPresent
			if staffIdx%3 == 1 && dayIdx%2 == 0 {
				status = active.WorkSessionStatusHomeOffice
			}

			// Build break records for this session
			breaks := buildBreaksForSession(checkIn, checkOut, staffIdx, dayIdx)

			// break_minutes = sum of all break durations (cached total)
			breakMins := 0
			for _, b := range breaks {
				breakMins += b.DurationMinutes
			}

			session := &active.WorkSession{
				StaffID:      staff.ID,
				Date:         d,
				Status:       status,
				CheckInTime:  checkIn,
				CheckOutTime: &checkOut,
				BreakMinutes: breakMins,
				CreatedBy:    staff.ID,
			}
			session.CreatedAt = time.Now()
			session.UpdatedAt = time.Now()

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
			if err != nil {
				return fmt.Errorf("failed to upsert work session for staff %d on %s: %w",
					staff.ID, d.Format("2006-01-02"), err)
			}

			s.result.WorkSessions = append(s.result.WorkSessions, session)

			// Insert break records for this session
			for _, b := range breaks {
				b.SessionID = session.ID
				b.CreatedAt = time.Now()
				b.UpdatedAt = time.Now()

				_, err := s.tx.NewInsert().Model(b).
					ModelTableExpr("active.work_session_breaks").
					On("CONFLICT DO NOTHING").
					Returning(SQLBaseColumns).
					Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to insert break for session %d: %w",
						session.ID, err)
				}

				s.result.WorkSessionBreaks = append(s.result.WorkSessionBreaks, b)
			}
		}
	}

	if s.verbose {
		log.Printf("Created %d work sessions with %d breaks",
			len(s.result.WorkSessions), len(s.result.WorkSessionBreaks))
	}
	return nil
}

// buildBreaksForSession generates 1-2 realistic break records for a work session.
// Morning break: ~10:00 for 15-20 min (not every day)
// Lunch break: ~12:30 for 30-45 min (§4 ArbZG: ≥30min for >6h, ≥45min for >9h)
func buildBreaksForSession(checkIn, checkOut time.Time, staffIdx, dayIdx int) []*active.WorkSessionBreak {
	var breaks []*active.WorkSessionBreak
	netHours := checkOut.Sub(checkIn).Hours()
	day := checkIn.Truncate(24 * time.Hour)

	// Morning break: ~10:00-10:20, only some staff/days
	if staffIdx%2 == 0 || dayIdx%3 == 0 {
		morningStart := day.Add(10*time.Hour + time.Duration(staffIdx*5)*time.Minute)
		morningDur := 15 + (dayIdx%2)*5 // 15 or 20 min
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
	// Add some variation
	lunchDur += (dayIdx * 3) % 7 // +0 to +6 min

	lunchEnd := lunchStart.Add(time.Duration(lunchDur) * time.Minute)

	breaks = append(breaks, &active.WorkSessionBreak{
		StartedAt:       lunchStart,
		EndedAt:         &lunchEnd,
		DurationMinutes: lunchDur,
	})

	return breaks
}
